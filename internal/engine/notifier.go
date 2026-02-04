package engine

import "sync"

type Notifier interface {
	Notify(title, message string) error
}

var (
	notifyOnce sync.Once
	notifyInst Notifier
)

func notifier() Notifier {
	notifyOnce.Do(func() {
		notifyInst = newNotifier()
	})

	return notifyInst
}
