---@class WeaveCtx
---@field run fun(self: WeaveCtx, cmd: string): { ok: boolean, code: integer, out: string, err: string }
---@field run fun(self: WeaveCtx, host: string, cmd: string): { ok: boolean, code: integer, out: string, err: string }
---@field sync fun(self: WeaveCtx, src: string, dst: string): { ok: boolean, code: integer, out: string, err: string }
---@field fetch fun(self: WeaveCtx, src: string, dst: string): { ok: boolean, code: integer, out: string, err: string }
---@field log fun(self: WeaveCtx, level: string, msg: string, fields?: table): nil
---@field notify fun(self: WeaveCtx, title: string, message: string): nil

---@alias TaskFn fun(ctx: WeaveCtx): nil
---@alias TaskOpts { depends?: string[] }

---@overload fun(name: string, fn: TaskFn)
---@overload fun(name: string, opts: TaskOpts, fn: TaskFn)
function task(name, opts, fn) end
