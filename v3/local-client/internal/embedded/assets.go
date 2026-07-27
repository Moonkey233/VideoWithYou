package embedded

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// extension is populated from extension/dist by scripts/build.ps1 before the
// Windows executable is compiled.
//
//go:embed extension
var extensionFiles embed.FS

func ExtractExtension(destination string) error {
	return fs.WalkDir(extensionFiles, "extension", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative := strings.TrimPrefix(path, "extension")
		relative = strings.TrimPrefix(relative, "/")
		target := destination
		if relative != "" {
			target = filepath.Join(destination, filepath.FromSlash(relative))
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		data, err := extensionFiles.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
}
