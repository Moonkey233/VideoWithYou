package adapter

import (
	"log"
	"sync"
	"time"

	"videowithyou/v2/local-client/internal/bridge"
	"videowithyou/v2/local-client/internal/model"
)

type BrowserAdapter struct {
	log       *log.Logger
	host      bridge.Host
	followURL bool

	mu    sync.RWMutex
	state model.PlayerState
}

func NewBrowserAdapter(host bridge.Host, logger *log.Logger, followURL bool) *BrowserAdapter {
	if logger == nil {
		logger = log.Default()
	}
	return &BrowserAdapter{
		log:       logger,
		host:      host,
		followURL: followURL,
	}
}

func (b *BrowserAdapter) Name() string { return "browser" }

func (b *BrowserAdapter) SetFollowURL(enabled bool) {
	b.followURL = enabled
}

func (b *BrowserAdapter) UpdatePlayerState(state model.PlayerState) {
	b.mu.Lock()
	state.UpdatedAt = time.Now()
	b.state = state
	b.mu.Unlock()
}

func (b *BrowserAdapter) GetState() (model.PlayerState, bool) {
	b.mu.RLock()
	state := b.state
	b.mu.RUnlock()
	if state.UpdatedAt.IsZero() {
		return model.PlayerState{}, false
	}
	return state, true
}

func (b *BrowserAdapter) ApplyState(state model.ApplyState) error {
	payload := map[string]any{
		"type":    "apply_state",
		"payload": state,
	}
	return b.host.Send(payload)
}

func (b *BrowserAdapter) Navigate(url string) error {
	if !b.followURL {
		return nil
	}
	payload := map[string]any{
		"type": "navigate",
		"payload": map[string]any{
			"url": url,
		},
	}
	return b.host.Send(payload)
}
