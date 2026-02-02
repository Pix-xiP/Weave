-- This is a Weavefile for use with the `weave` tool.
task("build", function(ctx)
	ctx:run("go build -o weave ./cmd/weave/main.go")
end)

task("rebuild", function(ctx)
	ctx:run("cp weave weave.old")
	local r = ctx:run("go build -o weave ./cmd/weave/main.go")
	if not r.ok then
		ctx:run("mv weave.old weave")
		ctx:log("error", "failed to build weave, restoring old weave")
		ctx:log("error", "output", { error = r.err })
	else
		ctx:run("rm weave.old")
		ctx:log("info", "weave rebuilt")
	end
end)
