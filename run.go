// Package run provides mechanisms to standardize command app execution.
package run

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
)

// Executor is a functional interface that defines an executable command.
//
// It takes a context, an output writer, and returns an error (if any occurred).
type Executor func(ctx context.Context, out io.Writer) error

type command struct {
	Executor

	flags *flag.FlagSet
}

// App is a runnable application configuration.
type App struct {
	name     string
	commands map[string]command

	out    io.Writer
	errOut io.Writer
}

// NewApp returns an initialized App.
func NewApp(name string, out io.Writer, errOut io.Writer) *App {
	return &App{
		name:     name,
		commands: make(map[string]command),

		out:    out,
		errOut: errOut,
	}
}

// SetCommand sets a command for the given name, executor, and flags.
func (a *App) SetCommand(name string, exec Executor, flags *flag.FlagSet) {
	a.commands[name] = command{exec, flags}
}

// CommandNames returns the names of the set commands.
func (a *App) CommandNames() []string {
	names := make([]string, 0, len(a.commands))

	for name := range a.commands {
		names = append(names, name)
	}

	return names
}

// Run takes a context and arguments, runs the expected command, and returns an
// exit code.
func (a *App) Run(ctx context.Context, arguments []string) int {
	if len(arguments) == 0 {
		arguments = os.Args[:]
	}

	if len(os.Args) > 1 && a.name == arguments[0] {
		arguments = arguments[1:]
	}

	if len(a.commands) == 0 || len(arguments) == 0 {
		return a.printErr(a.unexpectedCommandErr(), false)
	}

	commandName := arguments[0]
	cmd, ok := a.commands[commandName]
	if !ok {
		return a.printErr(a.unexpectedCommandErr(), false)
	}

	if err := cmd.flags.Parse(arguments[1:]); err != nil {
		return a.printErr(err, false)
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	fmt.Fprintf(a.out, "Starting %s...\n", commandName)

	errChan := make(chan error)
	go func() {
		errChan <- cmd.Executor(ctx, a.out)
	}()

	select {
	case err := <-errChan:
		returnCode := 0

		if err != nil {
			a.printErr(err, true)
			returnCode = 1
		}

		stop()

		return returnCode
	case <-ctx.Done():
		fmt.Fprintf(a.out, "\n\nDone.\n")
		stop()
	}

	return 0
}

func (a *App) unexpectedCommandErr() error {
	expected := a.CommandNames()

	var msg string
	switch {
	case len(expected) >= 2:
		last := expected[len(expected)-1]
		expected = expected[:len(expected)-1]
		withCommas := strings.Join(expected, "', '")

		msg = fmt.Sprintf("expected '%s' or '%s'", withCommas, last)
	case len(expected) == 1:
		msg = fmt.Sprintf("expected '%s'", expected[0])
	case len(expected) == 0:
		msg = "no commands"
	}

	return errors.New(msg)
}

func (a *App) printErr(err error, pad bool) int {
	msgFmt := "Error: %v\n"

	if pad {
		msgFmt = "\n\n" + msgFmt
	}

	fmt.Fprintf(a.errOut, msgFmt, err)

	return 1
}
