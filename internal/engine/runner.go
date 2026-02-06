package engine

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

type TaskName string

type Runner interface {
	Run(name TaskName) error
}

func RunGraphParallel(r Runner, deps map[TaskName][]TaskName, maxWorkers int) error {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}

	depsCount := map[TaskName]int{}
	dependents := map[TaskName][]TaskName{}
	all := map[TaskName]struct{}{}

	for t, ds := range deps {
		all[t] = struct{}{}

		depsCount[t] = len(ds)
		for _, d := range ds {
			all[d] = struct{}{}
			dependents[d] = append(dependents[d], t)
		}
	}
	for d := range dependents {
		sortTaskNames(dependents[d])
	}

	for t := range all {
		if _, ok := depsCount[t]; !ok {
			depsCount[t] = 0
		}
	}

	ready := make([]TaskName, 0, len(all))
	for t := range all {
		if depsCount[t] == 0 {
			ready = append(ready, t)
		}
	}
	sortTaskNames(ready)

	processed := 0

	for len(ready) > 0 {
		batch := ready
		ready = nil

		if err := runBatch(r, batch, maxWorkers); err != nil {
			return err
		}

		processed += len(batch)
		for _, t := range batch {
			for _, dep := range dependents[t] {
				depsCount[dep]--
				if depsCount[dep] == 0 {
					ready = append(ready, dep)
				}
			}
		}
		sortTaskNames(ready)
	}

	if processed != len(all) {
		if cycle := detectCycle(deps); len(cycle) > 0 {
			return errors.New("cycle: " + strings.Join(cycle, " -> "))
		}
		return errors.New("dependency cycle detected")
	}

	return nil
}

func runBatch(r Runner, batch []TaskName, maxWorkers int) error {
	if len(batch) == 0 {
		return nil
	}

	sem := make(chan struct{}, maxWorkers)

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)

	for _, t := range batch {
		wg.Add(1)

		go func(task TaskName) {
			defer wg.Done()

			sem <- struct{}{}

			defer func() { <-sem }()

			if err := r.Run(task); err != nil {
				mu.Lock()

				if firstErr == nil {
					firstErr = err
				}

				mu.Unlock()
			}
		}(t)
	}

	wg.Wait()

	return firstErr
}

func sortTaskNames(names []TaskName) {
	sort.Slice(names, func(i, j int) bool {
		return names[i] < names[j]
	})
}

func detectCycle(deps map[TaskName][]TaskName) []string {
	const (
		unvisited = iota
		visiting
		done
	)

	state := map[TaskName]int{}
	stack := []TaskName{}

	var dfs func(TaskName) []string
	dfs = func(n TaskName) []string {
		state[n] = visiting
		stack = append(stack, n)

		for _, d := range deps[n] {
			if state[d] == visiting {
				return buildCycle(stack, d)
			}
			if state[d] == unvisited {
				if cycle := dfs(d); len(cycle) > 0 {
					return cycle
				}
			}
		}

		stack = stack[:len(stack)-1]
		state[n] = done
		return nil
	}

	nodes := make([]TaskName, 0, len(deps))
	for n := range deps {
		nodes = append(nodes, n)
	}
	sortTaskNames(nodes)

	for _, n := range nodes {
		if state[n] == unvisited {
			if cycle := dfs(n); len(cycle) > 0 {
				return cycle
			}
		}
	}

	return nil
}

func buildCycle(stack []TaskName, target TaskName) []string {
	idx := -1
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return []string{string(target), string(target)}
	}
	cycle := make([]string, 0, len(stack)-idx+1)
	for _, n := range stack[idx:] {
		cycle = append(cycle, string(n))
	}
	cycle = append(cycle, string(target))
	return cycle
}
