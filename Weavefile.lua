-- This is a Weavefile for use with the `weave` tool.
task("build", { depends = { "test" } }, function(ctx)
	ctx:run("go build -o weave ./cmd/weave/main.go")
	ctx:notify("Weave", "weave has finished running build")
end)

task("rebuild", { depends = { "test" } }, function(ctx)
	ctx:run("cp weave weave.old")

	local r = ctx:run("go build -o weave ./cmd/weave/main.go")
	if not r.ok then
		ctx:run("mv weave.old weave")
		ctx:log("error", "failed to build weave, restoring old weave")
		ctx:log("error", "output", { error = r.err })
	else
		ctx:run("rm weave.old")
	end

	ctx:notify("Weave", "weave has finished running rebuild")
end)

task("test", function(ctx)
	local r = ctx:run("go test ./...")
	if not r.ok then
		ctx:log("error", "weave tests failed")
		ctx:log("error", "output", { error = r.err })
	else
		ctx:log("info", "weave tests passed", { output = r.out })
	end

	ctx:notify("Weave", "weave has finished running tests")
end)
