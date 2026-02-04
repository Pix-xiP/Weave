//go:build !darwin && !linux

package engine

import "fmt"

type defaultNotifier struct{}

func newNotifier() Notifier {
	return defaultNotifier{}
}

func (defaultNotifier) Notify(title, message string) error {
	fmt.Printf("TITLE: %s\nMESSAGE: %s\n", title, message)
	return nil
}
