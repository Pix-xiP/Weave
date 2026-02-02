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

List available tasks:

```bash
weave tasks
```

**How to build:**

```bash
go build -o weave ./cmd/weave
```

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

Tasks can be run in parallel, using 2 workers to do this be default.

Examples of basic task operations can be found in the `./testfiles` directory.

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

## Events

Weave emits structured events for tasks and operations when run in debug mode:

- `task_start` / `task_end`
- `op_start` / `op_end`

There are also `message` events from `ctx:log` calls down in the lua.
