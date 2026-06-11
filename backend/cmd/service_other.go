//go:build !windows

package main

import (
	"errors"
	"log/slog"
)

func isRunningAsWindowsService() bool         { return false }
func runAsWindowsService(_ *slog.Logger) error { return nil }

func installWindowsService() error {
	return errors.New("Windows Service management is only supported on Windows")
}

func uninstallWindowsService() error {
	return errors.New("Windows Service management is only supported on Windows")
}
