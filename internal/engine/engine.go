package engine

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
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
	opt     Options
	bus     *events.Bus
	L       *lua.LState
	tasks   map[string]taskDef
	cfg     Config
	spinner *spinnerRenderer
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
	if !opts.Quiet && opts.LogFormat == log.TextFormatter {
		e.spinner = newSpinnerRenderer(os.Stderr)
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
	e.bus.Subscribe(func(ev events.Event) {
		switch ev.Type {
		case events.TaskStart:
			log.Debug("task start", "task", ev.Task)
		case events.TaskEnd:
			log.Debug("task end", "task", ev.Task, "ok", ev.Fields["ok"])
		case events.OpStart:
			log.Debug("op start", "task", ev.Task, "op", ev.Fields["op"], "host", ev.Fields["host"])

			if e.spinner != nil {
				e.spinner.Handle(ev)
			}
		case events.OpEnd:
			log.Debug("op end",
				"task", ev.Task,
				"op", ev.Fields["op"],
				"ok", ev.Fields["ok"],
				"code", ev.Fields["code"],
				"duration_ms", ev.Fields["duration_ms"],
			)

			if e.spinner != nil {
				e.spinner.Handle(ev)
			}
		case events.Message:
			l := log.WithPrefix("LUA")

			attrs, _ := ev.Fields["attrs"].([]any)
			if e.opt.LogFormat == log.TextFormatter {
				if multi, ok := attrStringAny(attrs, "error", "output"); ok && strings.Contains(multi, "\n") {
					fmt.Fprintln(os.Stderr, multi)
					return
				}
			}

			switch ev.Fields["level"] {
			case "debug":
				l.Debug(ev.Fields["msg"], attrs...)
			case "warn":
				l.Warn(ev.Fields["msg"], attrs...)
			case "error":
				l.Error(ev.Fields["msg"], attrs...)
			default:
				l.Info(ev.Fields["msg"], attrs...)
			}
		}
	})
}

func attrStringAny(attrs []any, keys ...string) (string, bool) {
	for i := 0; i+1 < len(attrs); i += 2 {
		k, ok := attrs[i].(string)
		if !ok || !stringInSlice(k, keys) {
			continue
		}

		if v, ok := attrs[i+1].(string); ok {
			return v, true
		}

		return "", false
	}

	return "", false
}

func stringInSlice(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
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
