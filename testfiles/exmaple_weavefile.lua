-- The config table is used to provide information to Weave context
config = {
	-- Hosts allow for definitions of remote hosts to run commands on
	hosts = {
		-- 'server' defines the name that can be passed to ctx:run() commands.
		-- The 'addr' is the address of the host to connect to.
		-- The 'user' is the user to connect as.
		server = { addr = "localhost", user = "pix" },
	},
}

-- Task definitions are a name that is registered with the Weave context
-- and a function that defines the task.
task("hello", function(ctx)
	-- Logging can be done through the engine via the ctx:log() function
	-- set the log level 'debug|info|warn|error'
	-- provide a message and then an optional table of key value pairs to structured logging.
	ctx:log("info", "starting log from LUA", { env = "dev", region = "aus" })

	-- Run commands with ctx:run(<command>)
	-- Collect outputs in a local return if desired
	-- Outputs return a table with the following keys:
	--	ok: boolean
	--	code: integer
	--	out: string
	--	err: string
	local r = ctx:run("echo 'hello from weave'")

	ctx:log("info", "results", { ok = r.ok, code = r.code, out = r.out, err = r.err })

	-- check the return code to see how execution went
	if not r.ok then
		-- log out errors on failure, or do further scripting
		ctx:log("error", "command failed", { err = r.err })
	else
		-- If it all went well and you need the output, print via 'out'
		ctx:log("info", "command succeeded", { out = r.out })
		-- You can of course skip the ctx engine completely and just print via lua!
		print(r.out)
	end
end)
