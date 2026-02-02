package engine

import (
	"errors"

	lua "github.com/yuin/gopher-lua"
)

type HostConfig struct {
	Addr string
	User string
}

type Config struct {
	Hosts map[string]HostConfig
}

func loadConfigFrom(L *lua.LState) (Config, error) {
	cfg := Config{}
	lv := L.GetGlobal("config")
	if lv == lua.LNil {
		return cfg, nil
	}

	tbl, ok := lv.(*lua.LTable)
	if !ok {
		return cfg, errors.New("config must be a table")
	}

	hosts, err := parseHosts(tbl)
	if err != nil {
		return cfg, err
	}

	if len(hosts) > 0 {
		cfg.Hosts = hosts
	}

	return cfg, nil
}

func parseHosts(cfg *lua.LTable) (map[string]HostConfig, error) {
	lv := cfg.RawGetString("hosts")
	if lv == lua.LNil {
		return nil, errors.New("no valid hosts found")
	}

	hostsTbl, ok := lv.(*lua.LTable)
	if !ok {
		return nil, errors.New("config.hosts must be a table")
	}

	hosts := map[string]HostConfig{}

	hostsTbl.ForEach(func(k, v lua.LValue) {
		if k.Type() != lua.LTString {
			return
		}

		hostTbl, ok := v.(*lua.LTable)
		if !ok {
			return
		}

		name := k.String()

		host := HostConfig{
			Addr: luaStringToString(hostTbl, "addr"),
			User: luaStringToString(hostTbl, "user"),
		}
		if host.Addr == "" {
			return
		}

		hosts[name] = host
	})

	if len(hosts) == 0 && hostsTbl.Len() > 0 {
		return nil, errors.New("config.hosts entries must be tables with at least addr")
	}

	return hosts, nil
}

func luaStringToString(tbl *lua.LTable, key string) string {
	lv := tbl.RawGetString(key)
	if s, ok := lv.(lua.LString); ok {
		return string(s)
	}

	return ""
}
