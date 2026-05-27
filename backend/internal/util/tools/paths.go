package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// archAssetsKey returns the assets/tools/<key>/ subdirectory name for the
// current GOOS+GOARCH. Panics on unsupported combinations because the app
// cannot operate without DB client binaries.
func archAssetsKey() string {
	switch runtime.GOOS {
	case "windows":
		if runtime.GOARCH == "amd64" {
			return "win-x64"
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			return "arm"
		}

		return "x64"
	}

	panic(fmt.Sprintf("unsupported OS/arch for DB client tools: %s/%s",
		runtime.GOOS, runtime.GOARCH))
}

// AssetsToolsDir returns the absolute path to assets/tools/<arch-key>/.
// In deployed / standalone mode the binary lives next to assets/tools/<arch>/,
// so the executable directory is checked first. Falls back to walking up from
// the current working directory (dev / Docker mode).
var AssetsToolsDir = sync.OnceValue(func() string {
	key := archAssetsKey()

	// Standalone / deployed: look next to the running executable first.
	if exe, err := os.Executable(); err == nil {
		path := filepath.Join(filepath.Dir(exe), "assets", "tools", key)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}

	// Dev / Docker: walk up from cwd looking for the directory.
	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("could not get cwd: %v", err))
	}

	candidate := cwd
	for {
		path := filepath.Join(candidate, "assets", "tools", key)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}

		parent := filepath.Dir(candidate)
		if parent == candidate {
			break
		}
		candidate = parent
	}

	panic(fmt.Sprintf("could not locate assets/tools/%s starting from executable dir or %s", key, cwd))
})
