package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	logger "github.com/charmbracelet/log"
	"github.com/pix-xip/go-command"

	"github.com/pix-xip/weave/internal/engine"
)

func main() {
	r := command.Root().Help("Weave is a tool for executing Weavefile's").
		Flags(func(f *flag.FlagSet) {
			f.String("f", "Weavefile.lua", "path to Weavefile")
			f.String("log-level", "info", "set the log level [debug|info|warn|error]")
			f.String("log-format", "json", "set the log format [json|text]")

			f.Bool("quiet", false, "disable all output")
			f.Bool("debug", false, "enable debug mode")
		})

	r.Action(Start)

	if err := r.Execute(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Start(ctx context.Context, fs *flag.FlagSet, args []string) error {
	file := command.Lookup[string](fs, "f")
	logLevel := command.Lookup[string](fs, "log-level")
	logFormat := command.Lookup[string](fs, "log-format")
	quiet := command.Lookup[bool](fs, "quiet")
	debug := command.Lookup[bool](fs, "debug")

	level, err := logger.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	var format logger.Formatter

	switch logFormat {
	case "json":
		format = logger.JSONFormatter
	case "text":
		format = logger.TextFormatter
	default:
		return fmt.Errorf("invalid log format: %s", logFormat)
	}

	if debug {
		level = log.DebugLevel
	}

	eng := engine.New(engine.Options{
		File:      file,
		LogLevel:  level,
		LogFormat: format,
		Quiet:     quiet,
	})

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
