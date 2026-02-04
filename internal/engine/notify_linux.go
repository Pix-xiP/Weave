//go:build linux

package engine

import (
	"context"
	"fmt"
	"os/exec"
)

type linuxNotifier struct {
	bin string
}

func newNotifier() Notifier {
	path, err := exec.LookPath("notify-send")
	if err != nil || path == "" {
		return defaultNotifier{}
	}

	return linuxNotifier{bin: path}
}

func (n linuxNotifier) Notify(title, message string) error {
	cmd := exec.CommandContext(context.Background(), n.bin, title, message)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("notify-send: %w", err)
	}

	return nil
}

// TODO: consider dunstify and kdialog fallbacks for richer Linux desktop support.

type defaultNotifier struct{}

func (defaultNotifier) Notify(title, message string) error {
	fmt.Printf("TITLE: %s\nMESSAGE: %s\n", title, message)
	return nil
}
