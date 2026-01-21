package adapter

import (
	"bufio"
	"fmt"
	"html"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"videowithyou/v2/local-client/internal/config"
	"videowithyou/v2/local-client/internal/model"
)

const mpcAvailableTTL = 2 * time.Second

type MPCAdapter struct {
	log    *log.Logger
	cfg    config.MPCConfig
	client *http.Client

	mu                sync.Mutex
	lastState         model.PlayerState
	lastAvailable     time.Time
	lastControlAt     time.Time
	lastControlPaused bool
	hasControl        bool
}

func NewMPCAdapter(cfg config.MPCConfig, logger *log.Logger) *MPCAdapter {
	if logger == nil {
		logger = log.Default()
	}
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 800 * time.Millisecond
	}
	jar, _ := cookiejar.New(nil)
	return &MPCAdapter{
		log:    logger,
		cfg:    cfg,
		client: &http.Client{Timeout: timeout, Jar: jar},
	}
}

func (m *MPCAdapter) Name() string { return "mpc" }

func (m *MPCAdapter) IsAvailable() bool {
	m.mu.Lock()
	last := m.lastAvailable
	m.mu.Unlock()
	if !last.IsZero() && time.Since(last) < mpcAvailableTTL {
		return true
	}
	_, ok := m.fetchVariables()
	if ok {
		m.mu.Lock()
		m.lastAvailable = time.Now()
		m.mu.Unlock()
	}
	return ok
}

func (m *MPCAdapter) SetFollowURL(_ bool) {}

func (m *MPCAdapter) UpdatePlayerState(_ model.PlayerState) {}

func (m *MPCAdapter) GetState() (model.PlayerState, bool) {
	vars, ok := m.fetchVariables()
	if !ok {
		return model.PlayerState{}, false
	}
	state := m.parseState(vars)
	m.mu.Lock()
	m.lastState = state
	m.lastAvailable = time.Now()
	m.mu.Unlock()
	return state, true
}

func (m *MPCAdapter) ApplyState(state model.ApplyState) error {
	m.mu.Lock()
	last := m.lastState
	hasLast := !last.UpdatedAt.IsZero()
	lastControlAt := m.lastControlAt
	lastControlPaused := m.lastControlPaused
	hasControl := m.hasControl
	m.mu.Unlock()

	if state.PositionMs >= 0 {
		if !hasLast || math.Abs(float64(state.PositionMs-last.PositionMs)) > 50 {
			_ = m.sendCommand(m.cfg.Commands.Seek, map[string]string{
				"ms":       strconv.FormatInt(state.PositionMs, 10),
				"sec":      fmt.Sprintf("%.3f", float64(state.PositionMs)/1000.0),
				"hhmmss":   formatHMS(state.PositionMs, false),
				"hhmmssms": formatHMS(state.PositionMs, true),
			})
		}
	}

	shouldControl := true
	if hasControl && lastControlPaused == state.Paused && time.Since(lastControlAt) < 2*time.Second {
		shouldControl = false
	}
	if shouldControl {
		if state.Paused {
			if m.cfg.Commands.Pause != "" {
				_ = m.sendCommand(m.cfg.Commands.Pause, nil)
			} else if !hasLast || !last.Paused {
				_ = m.sendCommand(m.cfg.Commands.PlayPause, nil)
			}
		} else {
			if m.cfg.Commands.Play != "" {
				_ = m.sendCommand(m.cfg.Commands.Play, nil)
			} else if !hasLast || last.Paused {
				_ = m.sendCommand(m.cfg.Commands.PlayPause, nil)
			}
		}
		m.mu.Lock()
		m.lastControlAt = time.Now()
		m.lastControlPaused = state.Paused
		m.hasControl = true
		m.mu.Unlock()
	}

	m.mu.Lock()
	if hasLast {
		last.PositionMs = state.PositionMs
		last.Paused = state.Paused
		last.UpdatedAt = time.Now()
		m.lastState = last
	}
	m.mu.Unlock()

	return nil
}

func (m *MPCAdapter) Navigate(_ string) error {
	return nil
}

func (m *MPCAdapter) fetchVariables() (map[string]string, bool) {
	path := strings.TrimSpace(m.cfg.VariablesPath)
	if path == "" {
		path = "/variables.html"
	}
	body, err := m.get(path)
	if err != nil {
		return nil, false
	}
	vars := parseKeyValues(body)
	htmlVars := parseHTMLParagraphs(body)
	if len(vars) == 0 {
		return htmlVars, true
	}
	for key, value := range htmlVars {
		if _, ok := vars[key]; !ok {
			vars[key] = value
		}
	}
	return vars, true
}

func (m *MPCAdapter) parseState(vars map[string]string) model.PlayerState {
	now := time.Now()
	durationVal, hasDuration := parseNumber(vars, "duration", "duration_ms", "durationms")
	positionVal, hasPosition := parseNumber(vars, "position", "position_ms", "positionms", "pos")
	positionMs := int64(0)
	if hasPosition {
		positionMs = normalizeTime(positionVal, durationVal, hasDuration)
	}
	durationMs := int64(0)
	if hasDuration {
		durationMs = normalizeTime(durationVal, durationVal, true)
	}

	paused := true
	if value, ok := vars["paused"]; ok {
		paused = parseBool(value)
	} else if value, ok := vars["state"]; ok {
		switch strings.TrimSpace(value) {
		case "2":
			paused = false
		case "1":
			paused = true
		}
	} else if value, ok := vars["playstate"]; ok {
		paused = strings.TrimSpace(value) != "2"
	}

	rate := 1.0

	mediaURL := strings.TrimSpace(firstValue(vars, "filepath", "file", "filename"))
	title := strings.TrimSpace(firstValue(vars, "file", "filename", "title"))
	if title == "" && mediaURL != "" {
		title = filepath.Base(mediaURL)
	}
	if mediaURL != "" && !strings.HasPrefix(mediaURL, "http") && !strings.HasPrefix(mediaURL, "file://") {
		mediaURL = toFileURL(mediaURL)
	}

	return model.PlayerState{
		PositionMs: positionMs,
		DurationMs: durationMs,
		Paused:     paused,
		Rate:       rate,
		Media: model.MediaInfo{
			URL:   mediaURL,
			Title: title,
			Site:  "mpc",
			Attrs: map[string]string{},
		},
		UpdatedAt: now,
	}
}

func (m *MPCAdapter) sendCommand(template string, tokens map[string]string) error {
	template = strings.TrimSpace(template)
	if template == "" {
		return nil
	}
	spec := applyTokens(template, tokens)
	method, path, body := parseCommandSpec(spec)
	switch method {
	case http.MethodPost:
		_, err := m.post(path, body)
		return err
	default:
		_, err := m.get(path)
		return err
	}
}

func (m *MPCAdapter) get(path string) (string, error) {
	return m.do(http.MethodGet, path, "")
}

func (m *MPCAdapter) post(path, body string) (string, error) {
	return m.do(http.MethodPost, path, body)
}

func (m *MPCAdapter) do(method, path, body string) (string, error) {
	urlStr := buildURL(m.cfg.BaseURL, path)
	var reader io.Reader
	if method == http.MethodPost {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, urlStr, reader)
	if err != nil {
		return "", err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if m.cfg.Username != "" || m.cfg.Password != "" {
		req.SetBasicAuth(m.cfg.Username, m.cfg.Password)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("mpc http status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func buildURL(base, path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		base = "http://127.0.0.1:13579"
	}
	if path == "" {
		return base
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func applyTokens(template string, tokens map[string]string) string {
	if tokens == nil {
		return template
	}
	out := template
	for key, value := range tokens {
		out = strings.ReplaceAll(out, "{"+key+"}", value)
	}
	return out
}

func parseCommandSpec(spec string) (string, string, string) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return http.MethodGet, "", ""
	}
	upper := strings.ToUpper(spec)
	method := http.MethodGet
	switch {
	case strings.HasPrefix(upper, "POST "):
		method = http.MethodPost
		spec = strings.TrimSpace(spec[5:])
	case strings.HasPrefix(upper, "GET "):
		spec = strings.TrimSpace(spec[4:])
	}
	path := spec
	body := ""
	if parts := strings.SplitN(spec, "|", 2); len(parts) == 2 {
		path = strings.TrimSpace(parts[0])
		body = strings.TrimSpace(parts[1])
	}
	return method, path, body
}

func parseKeyValues(body string) map[string]string {
	result := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		result[key] = value
	}
	return result
}

func parseHTMLParagraphs(body string) map[string]string {
	result := make(map[string]string)
	if body == "" {
		return result
	}
	lower := strings.ToLower(body)
	i := 0
	for {
		p := strings.Index(lower[i:], "<p")
		if p < 0 {
			break
		}
		p += i
		idIdx := strings.Index(lower[p:], "id=")
		if idIdx < 0 {
			i = p + 2
			continue
		}
		idIdx += p
		quoteIdx := strings.IndexAny(lower[idIdx+3:], "\"'")
		if quoteIdx < 0 {
			i = p + 2
			continue
		}
		quoteIdx += idIdx + 3
		quoteChar := lower[quoteIdx]
		idStart := quoteIdx + 1
		idEndRel := strings.IndexByte(lower[idStart:], quoteChar)
		if idEndRel < 0 {
			i = p + 2
			continue
		}
		idEnd := idStart + idEndRel
		gtRel := strings.IndexByte(lower[idEnd:], '>')
		if gtRel < 0 {
			i = p + 2
			continue
		}
		gt := idEnd + gtRel
		endTagRel := strings.Index(lower[gt:], "</p>")
		if endTagRel < 0 {
			i = p + 2
			continue
		}
		endTag := gt + endTagRel
		key := strings.TrimSpace(body[idStart:idEnd])
		if key != "" {
			value := strings.TrimSpace(body[gt+1 : endTag])
			result[strings.ToLower(key)] = html.UnescapeString(value)
		}
		i = endTag + 4
	}
	return result
}

func parseNumber(vars map[string]string, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := vars[strings.ToLower(key)]; ok {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				return v, true
			}
		}
	}
	return 0, false
}

func parseBool(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func normalizeTime(value float64, duration float64, hasDuration bool) int64 {
	if value <= 0 {
		return 0
	}
	if hasDuration {
		if duration <= 10000 && value <= 10000 {
			return int64(value * 1000)
		}
		if duration > 10000 && value <= 10000 {
			return int64(value * 1000)
		}
	}
	if value <= 1000 {
		return int64(value * 1000)
	}
	return int64(value)
}

func firstValue(vars map[string]string, keys ...string) string {
	for _, key := range keys {
		if value, ok := vars[strings.ToLower(key)]; ok {
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return ""
}

func toFileURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.ToSlash(path)
	if strings.HasPrefix(path, "//") {
		return "file:" + path
	}
	if len(path) >= 2 && path[1] == ':' {
		path = "/" + path
	}
	u := url.URL{Scheme: "file", Path: path}
	return u.String()
}

func formatHMS(ms int64, withMillis bool) string {
	if ms < 0 {
		ms = 0
	}
	totalSeconds := ms / 1000
	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60
	if withMillis {
		return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms%1000)
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
