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
	File       string
	LogFormat  log.Formatter // "json" or "text"
	LogLevel   log.Level     // "debug", "info", "warn", "error"
	Quiet      bool
	DryRun     bool
	MaxWorkers int
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
			log.Debug("task start", "task", e.Task)
		case events.TaskEnd:
			log.Debug("task end", "task", e.Task, "ok", e.Fields["ok"])
		case events.OpStart:
			log.Debug("op start", "task", e.Task, "op", e.Fields["op"], "host", e.Fields["host"])
		case events.OpEnd:
			log.Debug("op end",
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
	registerDSLWithTasks(e.L, e.tasks)
}

func registerDSLWithTasks(L *lua.LState, tasks map[string]taskDef) {
	L.SetGlobal("task", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)

		var (
			opts *lua.LTable
			fn   *lua.LFunction
		)

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

		tasks[name] = taskDef{fn: fn, deps: deps}

		return 0
	}))
}

func (e *Engine) Load() error {
	// reset tasks for idempotent laods
	e.tasks = make(map[string]taskDef)
	e.cfg = Config{}
	registerDSLWithTasks(e.L, e.tasks)

	if err := e.L.DoFile(e.opt.File); err != nil {
		return fmt.Errorf("failure executing %s: %w", e.opt.File, err)
	}

	cfg, err := loadConfigFrom(e.L)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	e.cfg = cfg

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
	graph, err := e.depsGraph(name)
	if err != nil {
		return err
	}

	runner := engineRunner{engine: e}

	maxWorkers := e.opt.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 1
	}

	return RunGraphParallel(runner, graph, maxWorkers)
}

type engineRunner struct {
	engine *Engine
}

func (r engineRunner) Run(name TaskName) error {
	taskName := string(name)
	return r.engine.runTaskIsolated(taskName)
}

func (e *Engine) runTaskIsolated(taskName string) error {
	L := lua.NewState()
	defer L.Close()

	tasks := make(map[string]taskDef)
	registerDSLWithTasks(L, tasks)

	if err := L.DoFile(e.opt.File); err != nil {
		return fmt.Errorf("failure executing %s: %w", e.opt.File, err)
	}

	cfg, err := loadConfigFrom(L)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	def, ok := tasks[taskName]
	if !ok {
		return fmt.Errorf("unknown task %q", taskName)
	}

	ctx := NewCtx(L, e.bus)
	ctx.cfg = cfg
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

	err = L.CallByParam(lua.P{
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

	return err
}

func (e *Engine) depsGraph(root string) (map[TaskName][]TaskName, error) {
	if _, ok := e.tasks[root]; !ok {
		return nil, fmt.Errorf("unknown task %q", root)
	}

	graph := map[TaskName][]TaskName{}
	visited := map[string]bool{}

	var visit func(string) error

	visit = func(taskName string) error {
		if visited[taskName] {
			return nil
		}

		def, ok := e.tasks[taskName]
		if !ok {
			return fmt.Errorf("unknown dependency %q", taskName)
		}

		visited[taskName] = true

		deps := make([]TaskName, 0, len(def.deps))
		for _, dep := range def.deps {
			if _, ok := e.tasks[dep]; !ok {
				return fmt.Errorf("unknown dependency %q", dep)
			}

			deps = append(deps, TaskName(dep))
			if err := visit(dep); err != nil {
				return err
			}
		}

		graph[TaskName(taskName)] = deps

		return nil
	}

	if err := visit(root); err != nil {
		return nil, err
	}

	return graph, nil
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
