# Weave

Weave is a lightweight automation orchestrator: Lua describes intent, Go executes it with structured events and logging.

## Quick Start

Create a `Weavefile.lua` in your project:

```lua
-- Weavefile.lua
task("hello", function(ctx)
  ctx:log("info", "starting log from LUA", { env = "dev", credsfile="~/.creds"})

  local r = ctx:run("echo hello from weave")
  ctx:log("info", "results", { ok = r.ok, code = r.code, out = r.out, err = r.err })
end)
```

**Run it:**

```bash
weave run hello
```

```bash
weave tasks
```

**How to build:**

```bash
go build -o weave ./cmd/weave
```

List available tasks:

## Task API

Define tasks by name and function:

```lua
task("build", function(ctx)
  ctx:run("make -j")
end)
```

Add dependencies with an options table:

```lua
task("sync", function(ctx)
  ctx:sync("./", "server:/tmp/proj/")
end)

task("build", { depends = { "sync" } }, function(ctx)
  ctx:run("server", "go build ./tmp/proj")
end)
```

When you run a task, Weave executes all dependencies first, in order, and stops on the first failure.

## ctx Primitives

Everything runs through the `ctx` object:

```lua
ctx:run("go build .")                  -- local
ctx:run("server", "go build .")        -- remote via ssh

ctx:sync("./", "server:/tmp/proj/")                -- rsync upload
ctx:fetch("server:/tmp/proj/out.tar.gz", "./out/") -- rsync download

ctx:log("info", "message", { key = "value" })
```

**Notes:**

- `ctx:run(host, cmd)` uses `ssh user@addr -- sh -lc '<cmd>'` internally.
- `ctx:sync` / `ctx:fetch` are rsync-based. Use trailing `/` to copy contents, no trailing `/` to copy the directory itself.

## Host Config (optional)

You can define host aliases in your `Weavefile.lua`:

```lua
config = {
  hosts = {
    server = { addr = "buildbox", user = "pix" },
  },
}
```

These aliases work with `ctx:run("server", ...)` and `server:/path` in `ctx:sync` / `ctx:fetch`.

## Flags

```bash
weave --log-format text run hello
weave --dry-run run hello
```

## Events

Weave emits structured events for tasks and operations:

- `task_start` / `task_end`
- `op_start` / `op_end`
- `message` (from `ctx:log`)
