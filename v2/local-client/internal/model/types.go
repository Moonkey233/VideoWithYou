package model

import "time"

type MediaInfo struct {
    URL   string            `json:"url"`
    Title string            `json:"title"`
    Site  string            `json:"site"`
    Attrs map[string]string `json:"attrs"`
}

type PlayerState struct {
    PositionMs int64     `json:"position_ms"`
    DurationMs int64     `json:"duration_ms"`
    Paused     bool      `json:"paused"`
    Rate       float64   `json:"rate"`
    Media      MediaInfo `json:"media"`
    UpdatedAt  time.Time `json:"-"`
}

type ApplyState struct {
    PositionMs int64   `json:"position_ms"`
    Paused     bool    `json:"paused"`
    Rate       float64 `json:"rate"`
}