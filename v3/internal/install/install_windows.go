//go:build windows

package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func Install(enableAutostart bool) (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if strings.TrimSpace(localAppData) == "" {
		return "", fmt.Errorf("LOCALAPPDATA is not set")
	}
	installDir := filepath.Join(localAppData, "Programs", "VideoWithYou")
	if err := os.MkdirAll(installDir, 0o700); err != nil {
		return "", err
	}
	target := filepath.Join(installDir, "VideoWithYou.exe")
	source, err := os.Executable()
	if err != nil {
		return "", err
	}
	source, _ = filepath.Abs(source)
	target, _ = filepath.Abs(target)
	if !strings.EqualFold(filepath.Clean(source), filepath.Clean(target)) {
		if err := copyExecutable(source, target); err != nil {
			return "", err
		}
	}
	if enableAutostart {
		if err := setAutostart(target); err != nil {
			return "", err
		}
	}
	return target, nil
}

func copyExecutable(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func setAutostart(executable string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue("VideoWithYou", fmt.Sprintf("%q", executable))
}
