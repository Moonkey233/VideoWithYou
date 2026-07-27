//go:build !windows

package install

import "errors"

func Install(bool) (string, error) {
	return "", errors.New("self-install is only supported on Windows")
}
