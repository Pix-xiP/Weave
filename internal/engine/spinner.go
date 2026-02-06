package engine

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"

	"github.com/pix-xip/weave/internal/events"
)

type spinnerRenderer struct {
	mu  sync.Mutex
	ops map[string]*spinState
	out io.Writer
}

type spinState struct {
	label   string
	frames  []string
	fps     time.Duration
	index   int
	stopped chan struct{}
}

func newSpinnerRenderer(out io.Writer) *spinnerRenderer {
	return &spinnerRenderer{
		ops: make(map[string]*spinState),
		out: out,
	}
}

func (r *spinnerRenderer) Handle(e events.Event) {
	switch e.Type {
	case events.OpStart:
		r.handleStart(e)
	case events.OpEnd:
		r.handleEnd(e)
	}
}

func (r *spinnerRenderer) handleStart(e events.Event) {
	op, _ := e.Fields["op"].(string)
	if op != "sync" && op != "fetch" {
		return
	}

	label := fmt.Sprintf("%sing %s -> %s", op, strField(e.Fields, "src"), strField(e.Fields, "dst"))
	key := op + ":" + label

	r.mu.Lock()

	if _, ok := r.ops[key]; ok {
		r.mu.Unlock()
		return
	}

	spin := spinner.Pulse
	state := &spinState{
		label:   label,
		frames:  spin.Frames,
		fps:     spin.FPS,
		stopped: make(chan struct{}),
	}
	r.ops[key] = state
	r.mu.Unlock()

	go r.run(state)
}

func (r *spinnerRenderer) handleEnd(e events.Event) {
	op, _ := e.Fields["op"].(string)
	if op != "sync" && op != "fetch" {
		return
	}

	label := fmt.Sprintf("%sing %s -> %s", op, strField(e.Fields, "src"), strField(e.Fields, "dst"))
	key := op + ":" + label

	r.mu.Lock()

	state, ok := r.ops[key]
	if ok {
		delete(r.ops, key)
		close(state.stopped)
	}

	r.mu.Unlock()

	if ok {
		r.printf("\r%s done\n", state.label)
	}
}

func (r *spinnerRenderer) run(state *spinState) {
	ticker := time.NewTicker(state.fps)
	defer ticker.Stop()

	for {
		select {
		case <-state.stopped:
			return
		case <-ticker.C:
			frame := state.frames[state.index%len(state.frames)]
			state.index++
			r.printf("\r%s %s", state.label, frame)
		}
	}
}

func (r *spinnerRenderer) printf(format string, args ...any) {
	if r.out == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	fmt.Fprintf(r.out, format, args...)
}

func strField(fields map[string]any, key string) string {
	if v, ok := fields[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}
