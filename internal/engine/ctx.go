// Package engine implements the Lua DSL primitives for Weave
package engine

import (
	"bytes"
	"os/exec"
	"runtime"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/pix-xip/weave/internal/events"
)

type Ctx struct {
	L  *lua.LState
	ud *lua.LUserData

	bus events.Emitter
}

func NewCtx(L *lua.LState, bus events.Emitter) *Ctx {
	c := &Ctx{
		L:   L,
		bus: bus,
	}
	ud := L.NewUserData()
	ud.Value = c
	c.ud = ud

	meta := L.NewTypeMetatable("weave_ctx")
	L.SetField(meta, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"run": c.luaRun,
		"log": c.luaLog,
	}))

	L.SetMetatable(ud, meta)

	return c
}

// ctx:run("echo hi") -> { ok=true, code=0, out="...", err="..." }
func (c *Ctx) luaRun(L *lua.LState) int {
	// method call: arg1 is userdata, arg2 is first user arg
	cmdstr := L.CheckString(2)

	start := time.Now()

	c.bus.Emit(events.Event{
		Type:   events.OpStart,
		Time:   time.Now(),
		Task:   "run",
		Fields: map[string]any{"op": "run", "host": "local", "cmd": cmdstr},
	})

	var cmd *exec.Cmd

	// use a shell for convenience initially

	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", cmdstr)
	} else {
		// hotpath
		cmd = exec.Command("sh", "-lc", cmdstr)
	}

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0

	if err != nil {
		// best-effort exit code extraction:
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}

	dur := time.Since(start)
	c.bus.Emit(events.Event{
		Type: events.OpEnd,
		Time: time.Now(),
		Task: "run",
		Fields: map[string]any{
			"op":          "run",
			"host":        "local",
			"ok":          err == nil,
			"code":        code,
			"duration_ms": dur.Milliseconds(),
			"stdout_len":  len(stdout.String()),
			"stderr_len":  len(stderr.String()),
		},
	})

	res := L.NewTable()
	L.SetField(res, "ok", lua.LBool(err == nil))
	L.SetField(res, "code", lua.LNumber(code))
	L.SetField(res, "out", lua.LString(stdout.String()))
	L.SetField(res, "err", lua.LString(stderr.String()))
	L.Push(res)

	return 1
}

func (c *Ctx) luaLog(L *lua.LState) int {
	level := L.CheckString(2)
	msg := L.CheckString(3)

	var tbl *lua.LTable

	if L.GetTop() >= 4 {
		tbl = L.CheckTable(4)
	}

	attrs := []any{}

	if tbl != nil {
		tbl.ForEach(func(k, v lua.LValue) {
			attrs = append(attrs, k.String(), v.String())
		})
	}

	c.bus.Emit(events.Event{
		Type: events.Message,
		Time: time.Now(),
		Task: "log",
		Fields: map[string]any{
			"level": level,
			"msg":   msg,
			"attrs": attrs,
		},
	})
	return 0
}
