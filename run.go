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
	"reflect"
)

// Executor is a functional interface that defines an executable command.
//
// It takes a context, an output writer, and returns an error (if any occurred).
type Executor func(ctx context.Context, out io.Writer) error

// Flags defines an interface for command flags.
type Flags interface {
	Parse(arguments []string) error
	Args() []string
	PrintDefaults()
}

type command struct {
	name    string
	summary string
	Executor

	flags Flags // Flags for each command
}

// App is a runnable application configuration.
type App struct {
	name     string
	summary  string
	commands map[string]command

	flags Flags // Flags for the entire app (global)

	out    io.Writer
	errOut io.Writer
}

// NewApp returns an initialized App.
func NewApp(name string, summary string, flags Flags, out io.Writer, errOut io.Writer) *App {
	app := &App{
		name:     name,
		summary:  summary,
		commands: make(map[string]command),

		flags: flags,

		out:    out,
		errOut: errOut,
	}

	if flags == nil {
		app.flags = flag.CommandLine
	}

	app.setUsage(flags, "")

	app.SetCommand("help", "Display the help message", app.helpExecutor, nil)

	return app
}

// SetCommand sets a command for the given name, executor, and flags.
func (a *App) SetCommand(name string, summary string, exec Executor, flags Flags) {
	if flags == nil {
		flags = flag.CommandLine
	}

	if flags != a.flags {
		a.setUsage(flags, name)
	}

	a.commands[name] = command{name, summary, exec, flags}
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
		arguments = os.Args[1:]
	}

	a.flags.Parse(arguments)
	arguments = a.flags.Args()

	if len(a.commands) == 0 || len(arguments) == 0 {
		a.printUsage("")
		return 2
	}

	commandName := arguments[0]
	cmd, ok := a.commands[commandName]
	if !ok {
		return a.printUsageErr(commandName, fmt.Errorf("unknown command '%s'", commandName))
	}

	if err := cmd.flags.Parse(arguments[1:]); err != nil {
		return a.printErr(err, false)
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		stop()
	}()

	err := cmd.Executor(ctx, a.out)
	if err != nil && !errors.Is(err, context.Canceled) {
		return a.printErr(err, true)
	}

	return 0
}

// PrintUsage prints the usage to the app's error output.
//
// It's exposed so it can be called or assigned to a flag set's usage function.
func (a *App) PrintUsage(commandName string) func() {
	return func() {
		a.printUsage(commandName)
	}
}

// setUsage sets the `.Usage` field of the flags for a given command.
func (a *App) setUsage(flags Flags, commandName string) {
	if flags == nil {
		return
	}

	// TODO: Remove the use of reflection one we can use the type-system to
	// reliably detect types with specific fields, and set them.
	//
	// Until then, we'll HAVE to use reflection... :(
	flagsReflect := reflect.ValueOf(flags)
	if flagsReflect.IsValid() && flagsReflect.Kind() == reflect.Pointer {
		flagsReflect = flagsReflect.Elem()
	}

	usageReflect := flagsReflect.FieldByName("Usage")

	if !usageReflect.IsValid() || usageReflect.Kind() != reflect.Func {
		return
	}

	reflectFunc := reflect.ValueOf(a.PrintUsage(commandName))

	usageReflect.Set(reflectFunc)
}

func (a *App) printErr(err error, pad bool) int {
	msgFmt := "Error: %v\n"

	if pad {
		msgFmt = "\n" + msgFmt
	}

	fmt.Fprintf(a.errOut, msgFmt, err)

	return 1
}

func (a *App) fullCommandName(commandName string) string {
	name := a.name

	if commandName != "" && commandName != "help" {
		name = fmt.Sprintf("%s %s", name, commandName)
	}

	return name
}

func (a *App) printUsageErr(commandName string, err error) int {
	fmt.Fprintf(a.errOut, "%s: %v\n", a.fullCommandName(commandName), err)
	fmt.Fprintf(a.errOut, "Run '%s help' for usage.\n", a.name)

	return 1
}

func (a *App) printUsage(commandName string) {
	name := a.fullCommandName(commandName)
	command, hasCommand := a.commands[commandName]

	switch {
	case hasCommand && commandName != "help":
		fmt.Fprintf(a.errOut, "Usage: %s [arguments]\n\n", name)

		if command.summary != "" {
			fmt.Fprintln(a.errOut, command.summary)
		}

		command.flags.PrintDefaults()
	default:
		fmt.Fprintf(a.errOut, "Usage: %s <command> [arguments]\n\n", name)

		if a.summary != "" {
			fmt.Fprintln(a.errOut, a.summary)
		}

		fmt.Fprintf(a.errOut, "\nCommands:\n\n")

		for _, command := range a.commands {
			fmt.Fprintf(a.errOut, "\t%s\t%s\n", command.name, command.summary)
		}
	}
}

func (a *App) helpExecutor(ctx context.Context, out io.Writer) error {
	a.printUsage("help")

	return nil
}
