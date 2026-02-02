-- Weavefile for testing parallel tasks and dependency graphs

task("echo1", function(ctx)
	ctx:log("info", "running echo command", { echo = "echo1" })
	ctx:run("sleep 2")
	ctx:run("echo 'hello from echo1'")
end)

task("echo2", function(ctx)
	ctx:log("info", "running echo command", { echo = "echo2" })
	ctx:run("sleep 2")
	ctx:run("echo 'hello from echo1'")
end)

task("echo3", { depends = { "echo4" } }, function(ctx)
	ctx:log("info", "running echo command", { echo = "echo3" })
	ctx:run("sleep 2")
	ctx:run("echo 'hello from echo3'")
end)

task("echo4", function(ctx)
	ctx:log("info", "running echo command", { echo = "echo4" })
	ctx:run("sleep 2")
	ctx:run("echo 'hello from echo4'")
end)

task("echo5", function(ctx)
	ctx:log("info", "running echo command", { echo = "echo5" })
	ctx:run("sleep 2")
	ctx:run("echo 'hello from echo5'")
end)

task("echo6", function(ctx)
	ctx:log("info", "running echo command", { echo = "echo6" })
	ctx:run("sleep 2")
	ctx:run("echo 'hello from echo6'")
end)

task("echo", { depends = { "echo1", "echo2", "echo3", "echo4", "echo5", "echo6" } }, function(ctx)
	ctx:log("info", "running final echo command", { TheBiggestBean = "The Smollest Bean" })
	ctx:run("echo 'hello from the final echo'")
end)
