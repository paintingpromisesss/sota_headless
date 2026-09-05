//go:build windows

package main

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sys/windows/svc"
)

type winService struct {
	runFunc func(ctx context.Context) error
}

func (s *winService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- s.runFunc(ctx)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	slog.Info("Windows service started successfully")

	for {
		select {
		case err := <-errChan:
			if err != nil {
				slog.Error("Windows service error", "error", err)
				return false, 1
			}
			return false, 0
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				slog.Info("Windows service stop requested")
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				if err := <-errChan; err != nil {
					slog.Error("Windows service error on shutdown", "error", err)
					return false, 1
				}
				slog.Info("Windows service stopped gracefully")
				return false, 0
			default:
				slog.Warn(fmt.Sprintf("unexpected service control request: #%d", c))
			}
		}
	}
}

func runWindowsService(runFunc func(ctx context.Context) error) (bool, error) {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		return false, fmt.Errorf("failed to detect Windows service environment: %w", err)
	}
	if !isSvc {
		return false, nil
	}
	if err := svc.Run("sota-headless", &winService{runFunc: runFunc}); err != nil {
		return true, fmt.Errorf("Windows service failed: %w", err)
	}
	return true, nil
}
