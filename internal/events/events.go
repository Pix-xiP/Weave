package events

import "time"

type Type string

// Type Enums:
const (
	TaskStart Type = "task_start"
	TaskEnd   Type = "task_end"
	OpStart   Type = "op_start"
	OpEnd     Type = "op_end"
	Message   Type = "message"
)

type Event struct {
	Type   Type
	Time   time.Time
	Task   string
	Fields map[string]any
}

type Handler func(Event)

type Emitter interface {
	Emit(e Event)
	Subscribe(h Handler) (unsubscribe func())
}
