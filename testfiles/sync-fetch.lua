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

task("big-sync", function(ctx)
	ctx:log("info", "running big sync test command", { host = config.hosts.server.addr })
	ctx:run("dd if=/dev/urandom of=./bigfile bs=1M count=500")
	ctx:sync("./bigfile", "server:/home/pix/AdeptusCustodes/Lunar/Weave/bigfile_copy")
	ctx:run("rm ./bigfile")
	ctx:run("server", "rm /home/pix/AdeptusCustodes/Lunar/Weave/bigfile_copy")
	ctx:notify("Sync Done", "Bigfile Sync completed")
end)
