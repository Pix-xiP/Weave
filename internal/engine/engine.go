package engine

import (
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/charmbracelet/log"
	lua "github.com/yuin/gopher-lua"

	"github.com/pix-xip/weave/internal/events"
)

type Options struct {
	File      string
	LogFormat log.Formatter // "json" or "text"
	LogLevel  log.Level     // "debug", "info", "warn", "error"
	Quiet     bool
	DryRun    bool
}

type Engine struct {
	opt   Options
	bus   *events.Bus
	L     *lua.LState
	tasks map[string]*lua.LFunction
	cfg   Config
}

func New(opts Options) *Engine {
	if opts.Quiet {
		log.SetOutput(io.Discard)
	} else {
		log.SetOutput(os.Stderr)
		log.SetLevel(opts.LogLevel)
		log.SetFormatter(opts.LogFormat)
		log.SetTimeFormat(time.Kitchen)
	}

	e := &Engine{
		opt:   opts,
		bus:   events.NewBus(),
		L:     lua.NewState(),
		tasks: make(map[string]*lua.LFunction),
	}
	e.registerDSL()
	e.subscribe()

	return e
}

func (e *Engine) Close() {
	if e.L != nil {
		e.L.Close()
	}
}

func (e *Engine) subscribe() {
	e.bus.Subscribe(func(e events.Event) {
		switch e.Type {
		case events.TaskStart:
			log.Info("task start", "task", e.Task)
		case events.OpStart:
			log.Info("op start", "task", e.Task, "op", e.Fields["op"], "host", e.Fields["host"])
		case events.OpEnd:
			log.Info("op end",
				"task", e.Task,
				"op", e.Fields["op"],
				"ok", e.Fields["ok"],
				"code", e.Fields["code"],
				"duration_ms", e.Fields["duration_ms"],
			)
		case events.Message:
			l := log.WithPrefix("LUA")
			// If you want Lua user logs to land here
			switch e.Fields["level"] {
			case "debug":
				l.Debug(e.Fields["msg"], e.Fields["attrs"].([]any)...)
			case "warn":
				l.Warn(e.Fields["msg"], e.Fields["attrs"].([]any)...)
			case "error":
				l.Error(e.Fields["msg"], e.Fields["attrs"].([]any)...)
			default:
				l.Info(e.Fields["msg"], e.Fields["attrs"].([]any)...)
			}
		}
	})
}

func (e *Engine) registerDSL() {
	// task(name, fn)
	e.L.SetGlobal("task", e.L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		fn := L.CheckFunction(2)

		if name == "" {
			L.ArgError(1, "task name cannot be empty")
			return 1
		}

		e.tasks[name] = fn

		return 0
	}))
}

func (e *Engine) Load() error {
	// reset tasks for idempotent laods
	e.tasks = make(map[string]*lua.LFunction)
	e.cfg = Config{}

	if err := e.L.DoFile(e.opt.File); err != nil {
		return fmt.Errorf("failure executing %s: %w", e.opt.File, err)
	}

	if err := e.loadConfig(); err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	return nil
}

func (e *Engine) TaskNames() []string {
	out := make([]string, 0, len(e.tasks))
	for k := range e.tasks {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

func (e *Engine) Run(name string) error {
	fn, ok := e.tasks[name]
	if !ok {
		return fmt.Errorf("unknown task %q", name)
	}

	ctx := NewCtx(e.L, e.bus)
	ctx.cfg = e.cfg
	ctx.dryRun = e.opt.DryRun

	// call fn(ctx)
	if err := e.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    0,
		Protect: true,
	}, ctx.ud); err != nil {
		return err
	}

	return nil
}
