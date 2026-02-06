package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/log"
)

func TestEngineRunWithDeps(t *testing.T) {
	dir := t.TempDir()
	weavefile := filepath.Join(dir, "Weavefile.lua")
	if err := os.WriteFile(weavefile, []byte(`
task("sync", function(ctx)
  ctx:log("info", "sync")
end)

task("build", { depends = {"sync"} }, function(ctx)
  ctx:log("info", "build")
end)

task("release", { depends = {"build"} }, function(ctx)
  ctx:log("info", "release")
end)
`), 0o600); err != nil {
		t.Fatalf("write Weavefile: %v", err)
	}

	e := New(Options{
		File:      weavefile,
		LogFormat: log.TextFormatter,
		LogLevel:  log.DebugLevel,
		Quiet:     true,
	})
	if err := e.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := e.Run("release"); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
