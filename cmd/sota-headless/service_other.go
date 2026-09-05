//go:build !windows

package main

import "context"

func runWindowsService(_ func(ctx context.Context) error) (bool, error) {
	return false, nil
}
