package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/pix-xip/go-command"

	"github.com/pix-xip/weave/internal/engine"
)

var Version string

func main() {
	r := command.Root().Help("Weave is a tool for executing Weavefile's").
		Flags(func(f *flag.FlagSet) {
			f.String("f", "Weavefile.lua", "path to Weavefile")
			f.String("log-level", "info", "set the log level [debug|info|warn|error]")
			f.String("log-format", "text", "set the log format [json|text]")
			f.Bool("dry-run", false, "emit events without executing operations")
			f.Int("workers", 2, "max parallel tasks to run")

			f.Bool("quiet", false, "disable all output")
			f.Bool("debug", false, "enable debug mode")
		})

	r.Action(Start)

	r.SubCommand("version").Help("Prints the version").
		Action(func(ctx context.Context, fs *flag.FlagSet, args []string) error {
			fmt.Println("Weave version", Version)
			return nil
		})

	if err := r.Execute(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func makeOpts(fs *flag.FlagSet) (engine.Options, error) {
	level, err := log.ParseLevel(command.Lookup[string](fs, "log-level"))
	if err != nil {
		return engine.Options{}, fmt.Errorf("invalid log level: %w", err)
	}

	if command.Lookup[bool](fs, "debug") {
		level = log.DebugLevel
	}

	var format log.Formatter

	switch command.Lookup[string](fs, "log-format") {
	case "json":
		format = log.JSONFormatter
	case "text":
		format = log.TextFormatter
	default:
		return engine.Options{}, fmt.Errorf("invalid log format: %s",
			command.Lookup[string](fs, "log-format"))
	}

	return engine.Options{
		File:       command.Lookup[string](fs, "f"),
		LogLevel:   level,
		LogFormat:  format,
		Quiet:      command.Lookup[bool](fs, "quiet"),
		DryRun:     command.Lookup[bool](fs, "dry-run"),
		MaxWorkers: command.Lookup[int](fs, "workers"),
	}, nil
}

func Start(ctx context.Context, fs *flag.FlagSet, args []string) error {
	opts, err := makeOpts(fs)
	if err != nil {
		return err
	}

	eng := engine.New(opts)

	if len(args) == 0 {
		fs.Usage()
		os.Exit(0)
	}

	switch args[0] {
	case "tasks":
		if err := eng.Load(); err != nil {
			return fmt.Errorf("load error: %w", err)
		}

		for _, name := range eng.TaskNames() {
			fmt.Println(name)
		}
	case "run":
		if len(args) < 2 {
			return errors.New("missing task name")
		}

		taskName := args[1]

		if err := eng.Load(); err != nil {
			return fmt.Errorf("load error: %w", err)
		}

		if err := eng.Run(taskName); err != nil {
			return fmt.Errorf("run error: %w", err)
		}
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}

	return nil
}
