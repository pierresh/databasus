package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceName        = "Databasus"
	serviceDisplayName = "Databasus Backup Service"
	serviceDescription = "Databasus database backup service — runs scheduled backups automatically on Windows Server."
)

// databausService implements svc.Handler for the Windows Service Control Manager.
type databausService struct {
	log *slog.Logger
}

func (s *databausService) Execute(_ []string, req <-chan svc.ChangeRequest, statResp chan<- svc.Status) (bool, uint32) {
	statResp <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	go runApp(s.log)

	for c := range req {
		switch c.Cmd {
		case svc.Interrogate:
			statResp <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			statResp <- svc.Status{State: svc.StopPending}
			close(serviceShutdown)
			return false, 0
		}
	}

	return false, 0
}

func isRunningAsWindowsService() bool {
	ok, err := svc.IsWindowsService()
	return err == nil && ok
}

func runAsWindowsService(log *slog.Logger) error {
	return svc.Run(serviceName, &databausService{log: log})
}

func installWindowsService() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	exePath, err := filepath.Abs(exe)
	if err != nil {
		return fmt.Errorf("resolve absolute path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager (run as administrator?): %w", err)
	}
	defer m.Disconnect()

	existing, err := m.OpenService(serviceName)
	if err == nil {
		existing.Close()
		return fmt.Errorf("service %q already exists — run --uninstall-service first to replace it", serviceName)
	}

	s, err := m.CreateService(serviceName, exePath, mgr.Config{
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
		StartType:   mgr.StartAutomatic,
	}, "--standalone")
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	fmt.Printf("Service %q installed successfully.\n", serviceName)
	fmt.Println("Start it now:  sc start Databasus")
	fmt.Println("Or open:       services.msc → find Databasus → Start")

	return nil
}

func uninstallWindowsService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager (run as administrator?): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %q not found", serviceName)
	}
	defer s.Close()

	status, err := s.Query()
	if err == nil && status.State != svc.Stopped {
		if _, stopErr := s.Control(svc.Stop); stopErr != nil {
			fmt.Printf("warning: could not stop service before deletion: %v\n", stopErr)
		}
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}

	fmt.Printf("Service %q uninstalled successfully.\n", serviceName)

	return nil
}
