package adapter

import "videowithyou/v2/local-client/internal/model"

type Endpoint interface {
    Name() string
    GetState() (model.PlayerState, bool)
    UpdatePlayerState(state model.PlayerState)
    ApplyState(state model.ApplyState) error
    Navigate(url string) error
    SetFollowURL(enabled bool)
}
