-- This is a Weavefile for use with the `weave` tool.

task("hello", function(ctx)
	ctx:log("info", "starting log from LUA", { env = "dev", region = "aus" })

	local r = ctx:run("echo hello from weave")

	ctx:log("info", "results", { ok = r.ok, code = r.code, out = r.out, err = r.err })
	if not r.ok then
		ctx:log("error", "command failed", { err = r.err })
	end
end)

task("listdir", function(ctx)
	ctx:run("ls -la")
end)
