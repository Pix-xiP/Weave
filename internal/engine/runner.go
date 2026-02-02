package engine

import (
	"errors"
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
	}

	if processed != len(all) {
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
