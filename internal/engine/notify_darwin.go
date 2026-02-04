//go:build darwin

package engine

import (
	"fmt"
	"os/exec"
	"strings"
)

type darwinNotifier struct{}

func newNotifier() Notifier {
	return darwinNotifier{}
}

func (darwinNotifier) Notify(title, message string) error {
	title = strings.ReplaceAll(title, `"`, `\"`)
	message = strings.ReplaceAll(message, `"`, `\"`)
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("osascript notify: %w", err)
	}

	return nil
}
