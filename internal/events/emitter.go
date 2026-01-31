package events

import "sync"

// A thread safe event emitter for logging

type Bus struct {
	mu   sync.RWMutex
	subs map[int]Handler
	next int
}

func NewBus() *Bus {
	return &Bus{subs: make(map[int]Handler)}
}

func (b *Bus) Subscribe(h Handler) func() {
	b.mu.Lock()
	id := b.next
	b.next++
	b.subs[id] = h
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		delete(b.subs, id)
		b.mu.Unlock()
	}
}

func (b *Bus) Emit(e Event) {
	b.mu.RLock()
	handlers := make([]Handler, 0, len(b.subs))

	for _, h := range b.subs {
		handlers = append(handlers, h)
	}

	b.mu.RUnlock()

	// Call without holding lock so handler can emit and unsubscribe safely
	for _, h := range handlers {
		h(e)
	}
}
