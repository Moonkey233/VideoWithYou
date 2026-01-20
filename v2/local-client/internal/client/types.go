package client

import "videowithyou/v2/local-client/internal/config"

type Role string

const (
	RoleNone     Role = ""
	RoleHost     Role = "host"
	RoleFollower Role = "follower"
)

type UIState struct {
	RoomCode        string   `json:"room_code"`
	Role            string   `json:"role"`
	MembersCount    int      `json:"members_count"`
	Endpoint        string   `json:"endpoint"`
	FollowURL       bool     `json:"follow_url"`
	LastError       string   `json:"last_error"`
	DisplayName     string   `json:"display_name"`
	HostDisplayName string   `json:"host_display_name"`
	LastSyncTime    string   `json:"last_sync_time"`
	RoomEvents      []string `json:"room_events"`
	ServerConnected bool     `json:"server_connected"`
}

type UIAction struct {
	Action      string         `json:"action"`
	RoomCode    string         `json:"room_code,omitempty"`
	Endpoint    string         `json:"endpoint,omitempty"`
	FollowURL   *bool          `json:"follow_url,omitempty"`
	Config      *config.Config `json:"config,omitempty"`
	DisplayName string         `json:"display_name,omitempty"`
}
