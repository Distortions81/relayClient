//go:build !windows
// +build !windows

package main

func enableVirtualTerminalProcessing() error {
	return nil
}
