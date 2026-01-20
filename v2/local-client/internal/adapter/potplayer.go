package adapter

import (
    "errors"
    "fmt"
    "log"
    "os/exec"
    "strings"
    "sync"
    "time"

    "videowithyou/v2/local-client/internal/config"
    "videowithyou/v2/local-client/internal/model"
)

type Hotkey struct {
    Modifiers []uint16
    Key       uint16
}

type PotPlayerAdapter struct {
    log  *log.Logger
    cfg  config.PotPlayerConfig

    mu          sync.Mutex
    lastState   model.PlayerState
    lastUpdated time.Time
}

func NewPotPlayerAdapter(cfg config.PotPlayerConfig, logger *log.Logger) *PotPlayerAdapter {
    if logger == nil {
        logger = log.Default()
    }
    return &PotPlayerAdapter{
        log: logger,
        cfg: cfg,
    }
}

func (p *PotPlayerAdapter) Name() string { return "potplayer" }

func (p *PotPlayerAdapter) SetFollowURL(_ bool) {}

func (p *PotPlayerAdapter) UpdatePlayerState(_ model.PlayerState) {}

func (p *PotPlayerAdapter) GetState() (model.PlayerState, bool) {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.lastUpdated.IsZero() {
        return model.PlayerState{}, false
    }

    state := p.lastState
    if !state.Paused {
        elapsed := time.Since(p.lastUpdated)
        state.PositionMs += int64(float64(elapsed.Milliseconds()) * state.Rate)
    }
    state.UpdatedAt = time.Now()
    return state, true
}

func (p *PotPlayerAdapter) ApplyState(state model.ApplyState) error {
    p.mu.Lock()
    defer p.mu.Unlock()

    if state.PositionMs >= 0 {
        if err := p.seekTo(state.PositionMs); err != nil {
            p.log.Printf("potplayer seek failed: %v", err)
        } else {
            p.log.Printf("potplayer seek %dms", state.PositionMs)
        }
        p.lastState.PositionMs = state.PositionMs
    }

    if state.Paused != p.lastState.Paused {
        if err := p.sendHotkey(p.cfg.Hotkeys.PlayPause); err != nil {
            p.log.Printf("potplayer play/pause failed: %v", err)
        } else {
            p.log.Printf("potplayer toggle play/pause")
        }
        p.lastState.Paused = state.Paused
    }

    if state.Rate > 0 && state.Rate != p.lastState.Rate {
        if state.Rate > p.lastState.Rate {
            if err := p.sendHotkey(p.cfg.Hotkeys.RateUp); err != nil {
                p.log.Printf("potplayer rate up failed: %v", err)
            } else {
                p.log.Printf("potplayer rate up to %.2f", state.Rate)
            }
        } else {
            if err := p.sendHotkey(p.cfg.Hotkeys.RateDown); err != nil {
                p.log.Printf("potplayer rate down failed: %v", err)
            } else {
                p.log.Printf("potplayer rate down to %.2f", state.Rate)
            }
        }
        p.lastState.Rate = state.Rate
    }

    p.lastUpdated = time.Now()
    return nil
}

func (p *PotPlayerAdapter) Navigate(_ string) error {
    return nil
}

func (p *PotPlayerAdapter) seekTo(positionMs int64) error {
    if p.cfg.Path == "" {
        return errors.New("potplayer path not set")
    }
    seconds := positionMs / 1000
    arg := fmt.Sprintf("/seek=%d", seconds)
    cmd := exec.Command(p.cfg.Path, arg)
    return cmd.Run()
}

func (p *PotPlayerAdapter) sendHotkey(hotkey string) error {
    if strings.TrimSpace(hotkey) == "" {
        return errors.New("hotkey not configured")
    }
    parsed, err := parseHotkey(hotkey)
    if err != nil {
        return err
    }
    return sendHotkeyToPotPlayer(parsed)
}

func parseHotkey(value string) (Hotkey, error) {
    tokens := strings.Split(strings.ToUpper(value), "+")
    if len(tokens) == 0 {
        return Hotkey{}, errors.New("empty hotkey")
    }
    keys := make([]uint16, 0, len(tokens))
    for _, token := range tokens {
        token = strings.TrimSpace(token)
        if token == "" {
            continue
        }
        if key, ok := keyCodeFor(token); ok {
            keys = append(keys, key)
            continue
        }
        return Hotkey{}, fmt.Errorf("unsupported hotkey token: %s", token)
    }
    if len(keys) == 0 {
        return Hotkey{}, errors.New("empty hotkey")
    }
    if len(keys) == 1 {
        return Hotkey{Key: keys[0]}, nil
    }
    return Hotkey{Modifiers: keys[:len(keys)-1], Key: keys[len(keys)-1]}, nil
}

func keyCodeFor(token string) (uint16, bool) {
    if len(token) == 1 {
        r := token[0]
        if r >= 'A' && r <= 'Z' {
            return uint16(r), true
        }
        if r >= '0' && r <= '9' {
            return uint16(r), true
        }
    }
    switch token {
    case "CTRL", "CONTROL":
        return 0x11, true
    case "ALT":
        return 0x12, true
    case "SHIFT":
        return 0x10, true
    case "WIN", "META":
        return 0x5B, true
    case "SPACE":
        return 0x20, true
    case "ENTER":
        return 0x0D, true
    case "TAB":
        return 0x09, true
    case "UP":
        return 0x26, true
    case "DOWN":
        return 0x28, true
    case "LEFT":
        return 0x25, true
    case "RIGHT":
        return 0x27, true
    default:
        return 0, false
    }
}
