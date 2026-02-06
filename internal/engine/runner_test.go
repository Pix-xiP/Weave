package engine

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
)

type recordRunner struct {
	mu    sync.Mutex
	order []TaskName
	errs  map[TaskName]error
}

func (r *recordRunner) Run(name TaskName) error {
	r.mu.Lock()
	r.order = append(r.order, name)
	err := r.errs[name]
	r.mu.Unlock()
	return err
}

func TestRunGraphParallelLinear(t *testing.T) {
	deps := map[TaskName][]TaskName{
		"c": {"b"},
		"b": {"a"},
		"a": {},
	}
	r := &recordRunner{}
	if err := RunGraphParallel(r, deps, 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []TaskName{"a", "b", "c"}
	if !equalOrder(r.order, want) {
		t.Fatalf("order mismatch: got %v want %v", r.order, want)
	}
}

func TestRunGraphParallelFanOutStable(t *testing.T) {
	deps := map[TaskName][]TaskName{
		"build": {"sync"},
		"lint":  {"sync"},
		"sync":  {},
	}
	r := &recordRunner{}
	if err := RunGraphParallel(r, deps, 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.order) != 3 {
		t.Fatalf("unexpected order length: %d", len(r.order))
	}
	if r.order[0] != "sync" {
		t.Fatalf("expected sync first, got %v", r.order)
	}
	rest := []TaskName{r.order[1], r.order[2]}
	sort.Slice(rest, func(i, j int) bool { return rest[i] < rest[j] })
	if rest[0] != "build" || rest[1] != "lint" {
		t.Fatalf("unexpected fan-out order: %v", r.order)
	}
}

func TestRunGraphParallelCycle(t *testing.T) {
	deps := map[TaskName][]TaskName{
		"build":   {"release"},
		"release": {"build"},
	}
	r := &recordRunner{}
	err := RunGraphParallel(r, deps, 2)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "cycle:") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestRunGraphParallelStopsOnError(t *testing.T) {
	deps := map[TaskName][]TaskName{
		"b": {"a"},
		"a": {},
	}
	r := &recordRunner{errs: map[TaskName]error{"a": errors.New("fail")}}
	err := RunGraphParallel(r, deps, 2)
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(r.order) == 0 || r.order[0] != "a" {
		t.Fatalf("expected a to run first, got %v", r.order)
	}
	if len(r.order) > 1 {
		t.Fatalf("expected stop on first error, got %v", r.order)
	}
}

func equalOrder(got, want []TaskName) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
