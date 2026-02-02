-- Weavefile for testing sync and fetch commands.

config = {
	hosts = {
		server = { addr = "localhost", user = "pix" },
	},
}

task("sync-test", function(ctx)
	ctx:log("info", "running sync test command", { host = config.hosts.server.addr })
	local r = ctx:sync("./testfiles/syncfolder/", "server:/home/pix/AdeptusCustodes/Lunar/Weave/syncfolder/")
	if not r.ok then
		ctx:log("error", "sync failed", { err = r.err })
		return
	end
end)

task("fetch-test", { depends = { "sync-test" } }, function(ctx)
	ctx:log("info", "running fetch test command", { host = config.hosts.server.addr })
	local r = ctx:fetch("server:/home/pix/AdeptusCustodes/Lunar/Weave/syncfolder/", "./testfiles/fetchfolder/")
	if not r.ok then
		ctx:log("error", "fetch failed", { err = r.err })
		return
	end
end)
