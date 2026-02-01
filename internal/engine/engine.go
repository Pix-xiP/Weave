package engine

import (
	"errors"
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
	tasks map[string]taskDef
	cfg   Config
}

type taskDef struct {
	fn   *lua.LFunction
	deps []string
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
		tasks: make(map[string]taskDef),
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
		case events.TaskEnd:
			log.Info("task end", "task", e.Task, "ok", e.Fields["ok"])
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
	e.L.SetGlobal("task", e.L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)

		var opts *lua.LTable

		var fn *lua.LFunction

		switch L.GetTop() {
		case 2:
			fn = L.CheckFunction(2)
		case 3:
			opts = L.CheckTable(2)
			fn = L.CheckFunction(3)
		default:
			L.ArgError(1, "expected task(name, fn) or task(name, opts, fn)")
			return 1
		}

		if name == "" {
			L.ArgError(1, "task name cannot be empty")
			return 1
		}

		deps, err := parseTaskDeps(opts)
		if err != nil {
			L.ArgError(2, err.Error())
			return 1
		}

		e.tasks[name] = taskDef{fn: fn, deps: deps}

		return 0
	}))
}

func (e *Engine) Load() error {
	// reset tasks for idempotent laods
	e.tasks = make(map[string]taskDef)
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
	order, err := e.taskOrder(name)
	if err != nil {
		return err
	}

	for _, taskName := range order {
		def := e.tasks[taskName]
		ctx := NewCtx(e.L, e.bus)
		ctx.cfg = e.cfg
		ctx.dryRun = e.opt.DryRun

		start := time.Now()
		e.bus.Emit(events.Event{
			Type: events.TaskStart,
			Time: time.Now(),
			Task: taskName,
			Fields: map[string]any{
				"task": taskName,
			},
		})

		err := e.L.CallByParam(lua.P{
			Fn:      def.fn,
			NRet:    0,
			Protect: true,
		}, ctx.ud)

		e.bus.Emit(events.Event{
			Type: events.TaskEnd,
			Time: time.Now(),
			Task: taskName,
			Fields: map[string]any{
				"task":        taskName,
				"ok":          err == nil,
				"duration_ms": time.Since(start).Milliseconds(),
			},
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) taskOrder(name string) ([]string, error) {
	if _, ok := e.tasks[name]; !ok {
		return nil, fmt.Errorf("unknown task %q", name)
	}

	visited := map[string]bool{}
	inStack := map[string]bool{}
	order := []string{}

	var visit func(string) error

	visit = func(taskName string) error {
		if inStack[taskName] {
			return fmt.Errorf("dependency cycle detected at %q", taskName)
		}

		if visited[taskName] {
			return nil
		}

		def, ok := e.tasks[taskName]
		if !ok {
			return fmt.Errorf("unknown dependency %q", taskName)
		}

		inStack[taskName] = true

		for _, dep := range def.deps {
			if err := visit(dep); err != nil {
				return err
			}
		}

		inStack[taskName] = false

		visited[taskName] = true
		order = append(order, taskName)

		return nil
	}

	if err := visit(name); err != nil {
		return nil, err
	}

	return order, nil
}

func parseTaskDeps(opts *lua.LTable) ([]string, error) {
	if opts == nil {
		return nil, nil
	}

	lv := opts.RawGetString("depends")
	if lv == lua.LNil {
		return nil, nil
	}

	tbl, ok := lv.(*lua.LTable)
	if !ok {
		return nil, errors.New("depends must be a table of strings")
	}

	deps := []string{}

	tbl.ForEach(func(_, v lua.LValue) {
		if s, ok := v.(lua.LString); ok {
			deps = append(deps, string(s))
		}
	})

	if len(deps) != tbl.Len() {
		return nil, errors.New("depends must be a table of strings")
	}

	return deps, nil
}
