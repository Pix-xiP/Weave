// Package engine implements the Lua DSL primitives for Weave
package engine

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	lua "github.com/yuin/gopher-lua"

	"github.com/pix-xip/weave/internal/events"
)

type Ctx struct {
	L  *lua.LState
	ud *lua.LUserData

	bus events.Emitter
	cfg Config
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
		"run":   c.luaRun,
		"sync":  c.luaSync,
		"fetch": c.luaFetch,
		"log":   c.luaLog,
	}))

	L.SetMetatable(ud, meta)

	return c
}

// ctx:run("echo hi") -> { ok=true, code=0, out="...", err="..." }
func (c *Ctx) luaRun(L *lua.LState) int {
	// method call: arg1 is userdata, arg2 is first user arg
	top := L.GetTop()
	if top < 2 || top > 3 {
		L.ArgError(2, "expected ctx:run(cmd) or ctx:run(host, cmd)")
		return 1
	}

	var cmdstr string

	hostname := ""

	if top == 2 {
		cmdstr = L.CheckString(2)
	} else {
		hostname = L.CheckString(2)
		cmdstr = L.CheckString(3)
	}

	var cmd *exec.Cmd

	// use a shell for convenience initially
	if hostname == "" {
		// support only those with 'sh'
		cmd = exec.Command("sh", "-lc", cmdstr)
	} else {
		host, ok := c.cfg.Hosts[hostname]
		if !ok {
			L.ArgError(2, "unknown host: "+hostname)
			return 1
		}

		target := host.Addr
		if host.User != "" {
			target = host.User + "@" + host.Addr
		}

		remoteCmd := "sh -lc " + shellQuotePosix(cmdstr)
		cmd = exec.Command("ssh", target, "--", remoteCmd)
	}

	start := time.Now()

	c.bus.Emit(events.Event{
		Type:   events.OpStart,
		Time:   time.Now(),
		Task:   "run",
		Fields: map[string]any{"op": "run", "host": hostname, "cmd": cmdstr},
	})

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
			"host":        hostname,
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

func shellQuotePosix(s string) string {
	if s == "" {
		return "''"
	}

	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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

// ctx:sync(src, dst) -> { ok=true, code=0, out="", err="" }
func (c *Ctx) luaSync(L *lua.LState) int {
	src := L.CheckString(2)
	dst := L.CheckString(3)

	return c.luaRsync(L, "sync", src, dst)
}

// ctx:fetch(src, dst) -> { ok=true, code=0, out="", err="" }
func (c *Ctx) luaFetch(L *lua.LState) int {
	src := L.CheckString(2)
	dst := L.CheckString(3)

	return c.luaRsync(L, "fetch", src, dst)
}

func (c *Ctx) luaRsync(L *lua.LState, op, src, dst string) int {
	resolvedSrc, err := c.resolveRsyncPath(src)
	if err != nil {
		return c.luaRsyncError(L, err)
	}

	resolvedDst, err := c.resolveRsyncPath(dst)
	if err != nil {
		return c.luaRsyncError(L, err)
	}

	resolvedSrc = normalizeRsyncSource(op, src, resolvedSrc)

	if err := ensureLocalDest(resolvedDst); err != nil {
		return c.luaRsyncError(L, err)
	}

	start := time.Now()
	c.bus.Emit(events.Event{
		Type:   events.OpStart,
		Time:   time.Now(),
		Task:   op,
		Fields: map[string]any{"op": op, "src": src, "dst": dst},
	})

	log.Debugf("executing: rsync -az --delete %s %s", resolvedSrc, resolvedDst)
	cmd := exec.Command("rsync", "-az", "--delete", resolvedSrc, resolvedDst)

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	code := 0

	if err != nil {
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
		Task: op,
		Fields: map[string]any{
			"op":          op,
			"ok":          err == nil,
			"code":        code,
			"duration_ms": dur.Milliseconds(),
		},
	})

	errStr := ""
	if err != nil {
		errStr = stderr.String()
	}

	res := L.NewTable()

	L.SetField(res, "ok", lua.LBool(err == nil))
	L.SetField(res, "code", lua.LNumber(code))
	L.SetField(res, "out", lua.LString(stdout.String()))
	L.SetField(res, "err", lua.LString(errStr))
	L.Push(res)

	return 1
}

func (c *Ctx) luaRsyncError(L *lua.LState, err error) int {
	res := L.NewTable()
	L.SetField(res, "ok", lua.LBool(false))
	L.SetField(res, "code", lua.LNumber(1))
	L.SetField(res, "out", lua.LString(""))
	L.SetField(res, "err", lua.LString(err.Error()))
	L.Push(res)

	return 1
}

func ensureLocalDest(dst string) error {
	if dst == "" || isRemoteSpec(dst) {
		return nil
	}

	if strings.HasSuffix(dst, "/") {
		return mkdirAll(dst)
	}

	parent := filepath.Dir(dst)
	if parent == "." {
		return nil
	}

	return mkdirAll(parent)
}

func isRemoteSpec(path string) bool {
	return strings.Contains(path, ":")
}

func mkdirAll(path string) error {
	if path == "" {
		return nil
	}

	return os.MkdirAll(path, 0o750)
}

func (c *Ctx) resolveRsyncPath(path string) (string, error) {
	host, remotePath, ok := splitHostPath(path)
	if !ok {
		return path, nil
	}

	hostCfg, ok := c.cfg.Hosts[host]
	if !ok {
		return path, nil
	}

	target := hostCfg.Addr
	if hostCfg.User != "" {
		target = hostCfg.User + "@" + hostCfg.Addr
	}

	return target + ":" + remotePath, nil
}

func splitHostPath(path string) (string, string, bool) {
	parts := strings.SplitN(path, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	if parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

func normalizeRsyncSource(op, original, resolved string) string {
	if op != "sync" {
		return resolved
	}

	if _, _, ok := splitHostPath(original); ok {
		return resolved
	}

	info, err := os.Stat(original)
	if err != nil || !info.IsDir() {
		return resolved
	}

	if strings.HasSuffix(resolved, "/") {
		return resolved
	}

	return resolved + "/"
}
