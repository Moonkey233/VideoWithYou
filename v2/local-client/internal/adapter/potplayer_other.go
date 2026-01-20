//go:build !windows

package adapter

import "errors"

func sendHotkeyToPotPlayer(_ Hotkey) error {
    return errors.New("potplayer hotkeys not supported on this OS")
}