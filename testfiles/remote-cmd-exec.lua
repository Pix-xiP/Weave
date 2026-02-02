-- Weavefile for testing remote command execution

config = {
	hosts = {
		server = { addr = "localhost", user = "pix" },
	},
}

task("remote", function(ctx)
	ctx:log("info", "running remote command", { host = config.hosts.server.addr })
	local r = ctx:run("server", "ls -lah /home/pix/AdeptusCustodes/Lunar/Weave")
	if not r.ok then
		ctx:log("error", "remote command failed", { err = r.err })
		return
	end
end)
