// Package run provides mechanisms to standardize command app execution.
package run

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"reflect"
	"runtime"
)

// Executor is a functional interface that defines an executable command.
//
// It takes a context, an output writer, and returns an error (if any occurred).
type Executor func(ctx context.Context, arguments []string, out io.Writer) error

// Flags defines an interface for command flags.
type Flags interface {
	Parse(arguments []string) error
	Args() []string
	PrintDefaults()
	Output() io.Writer
	SetOutput(output io.Writer)
}

type command struct {
	name    string
	summary string
	Executor

	flags Flags // Flags for each command
}

// AppInfo describes information about an app.
type AppInfo struct {
	Name    string
	Summary string
	Version string
}

// app is a runnable application configuration.
type app struct {
	info AppInfo

	flags Flags // Flags for the entire app (global)

	out    io.Writer
	errOut io.Writer

	init func() error
}

// SingleCommandApp is a runnable application that only has one command.
type SingleCommandApp struct {
	*app

	exec Executor
}

// MultiCommandApp is a runnable application that has many commands.
type MultiCommandApp struct {
	*app

	commands map[string]command
}

// NewSingleCommandApp returns an initialized MultiCommandApp.
func NewSingleCommandApp(info AppInfo, exec Executor, flags Flags, out io.Writer, errOut io.Writer) *SingleCommandApp {
	if flags == nil {
		flags = flag.NewFlagSet(info.Name, flag.ExitOnError)
	}

	app := &SingleCommandApp{
		app: &app{
			info: info,

			flags: flags,

			out:    out,
			errOut: errOut,
		},

		exec: exec,
	}

	app.setUsage(flags)

	return app
}

// NewMultiCommandApp returns an initialized MultiCommandApp.
//
// The provided flags are global/shared among the app's commands.
func NewMultiCommandApp(info AppInfo, flags Flags, out io.Writer, errOut io.Writer) *MultiCommandApp {
	if flags == nil {
		flags = flag.NewFlagSet(info.Name, flag.ExitOnError)
	}

	app := &MultiCommandApp{
		app: &app{
			info: info,

			flags: flags,

			out:    out,
			errOut: errOut,
		},

		commands: make(map[string]command),
	}

	app.setUsage(flags, "")

	app.SetCommand("help", "Display the help message", app.helpExecutor, nil)

	return app
}

// SetCommand sets a command for the given name, executor, and flags.
//
// It returns an error if the provided flags have already been used for another
// command (or for the globals).
func (a *MultiCommandApp) SetCommand(name string, summary string, exec Executor, flags Flags) error {
	if !a.isUniqueFlagSet(flags) {
		return errors.New("provided flags are duplicate")
	}

	if flags == nil {
		flags = flag.NewFlagSet(a.fullCommandName(name), flag.ExitOnError)
	}

	a.setUsage(flags, name)

	a.commands[name] = command{name, summary, exec, flags}

	return nil
}

// CommandNames returns the names of the set commands.
func (a *MultiCommandApp) CommandNames() []string {
	names := make([]string, 0, len(a.commands))

	for name := range a.commands {
		names = append(names, name)
	}

	return names
}

// Run takes a context and arguments, runs the expected command, and returns an
// exit code.
func (a *SingleCommandApp) Run(ctx context.Context, arguments []string) int {
	arguments = a.initArgs(arguments)

	if err := a.initialize(); err != nil {
		return a.printErr(err, false)
	}

	return a.execute(ctx, a.exec, arguments)
}

// Run takes a context and arguments, runs the expected command, and returns an
// exit code.
func (a *MultiCommandApp) Run(ctx context.Context, arguments []string) int {
	arguments = a.initArgs(arguments)

	if len(a.commands) == 0 || len(arguments) == 0 {
		a.PrintHelp("")
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

	if err := a.initialize(); err != nil {
		return a.printErr(err, false)
	}

	return a.execute(ctx, cmd.Executor, arguments)
}

// OnInit takes an init function that is then called after initialization and
// before execution of a command.
func (a *app) OnInit(init func() error) {
	a.init = init
}

// PrintVersion prints the version to the app's standard output.
func (a *app) PrintVersion() {
	a.printVersion(false)
}

// PrintHelp prints the usage to the app's error output.
//
// It's exposed so it can be called or assigned to a flag set's usage function.
func (a *SingleCommandApp) PrintHelp() {
	printCommandUsage(a.info.Name, a.info.Summary, a.flags, a.errOut)
	fmt.Fprintln(a.errOut)
	a.printVersion(true)
}

// PrintHelp prints the usage to the app's error output.
//
// It's exposed so it can be called or assigned to a flag set's usage function.
func (a *MultiCommandApp) PrintHelp(commandName string) {
	a.printUsage(commandName)
	fmt.Fprintln(a.errOut)
	a.printVersion(true)
}

func (a *app) initArgs(arguments []string) []string {
	if len(arguments) == 0 {
		arguments = os.Args[1:]
	}

	a.flags.Parse(arguments)
	arguments = a.flags.Args()

	return arguments
}

func (a *app) initialize() error {
	if a.init == nil {
		return nil
	}

	return a.init()
}

func (a *app) execute(ctx context.Context, exec Executor, arguments []string) int {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	err := exec(ctx, arguments, a.out)
	if err != nil && !errors.Is(err, context.Canceled) {
		return a.printErr(err, true)
	}

	return 0
}

// setUsage sets the `.Usage` field of the flags for a given command.
func (a *SingleCommandApp) setUsage(flags Flags) {
	usageReflect := reflectFlagsUsage(flags)
	reflectFunc := reflect.ValueOf(a.PrintHelp)

	usageReflect.Set(reflectFunc)
}

// setUsage sets the `.Usage` field of the flags for a given command.
func (a *MultiCommandApp) setUsage(flags Flags, commandName string) {
	usageReflect := reflectFlagsUsage(flags)
	reflectFunc := reflect.ValueOf(func() { a.PrintHelp(commandName) })

	usageReflect.Set(reflectFunc)
}

func (a *MultiCommandApp) isUniqueFlagSet(flags Flags) bool {
	if flags == a.flags {
		return false
	}

	for _, command := range a.commands {
		if flags == command.flags {
			return false
		}
	}

	return true
}

func (a *app) printErr(err error, pad bool) int {
	msgFmt := "Error: %v\n"

	if pad {
		msgFmt = "\n" + msgFmt
	}

	fmt.Fprintf(a.errOut, msgFmt, err)

	return 1
}

func (a *app) fullCommandName(commandName string) string {
	name := a.info.Name

	if commandName != "" && commandName != "help" {
		name = fmt.Sprintf("%s %s", name, commandName)
	}

	return name
}

func (a *app) printVersion(toErr bool) {
	out := a.out
	if toErr {
		out = a.errOut
	}

	identifier := a.info.Name
	if a.info.Version != "" {
		identifier = fmt.Sprintf("%s %s", identifier, a.info.Version)
	}

	fmt.Fprintf(out, "%s (%s/%s)\n", identifier, runtime.GOOS, runtime.GOARCH)
}

func (a *app) printUsageErr(commandName string, err error) int {
	fmt.Fprintf(a.errOut, "%s: %v\n", a.fullCommandName(commandName), err)
	fmt.Fprintf(a.errOut, "Run '%s help' for usage.\n", a.info.Name)

	return 1
}

func (a *MultiCommandApp) printUsage(commandName string) {
	name := a.fullCommandName(commandName)
	command, hasCommand := a.commands[commandName]

	switch {
	case hasCommand && commandName != "help":
		printCommandUsage(name, command.summary, command.flags, a.errOut)
		printFlagDefaults(a.flags, true)
	default:
		fmt.Fprintf(a.errOut, "Usage: %s <command> [arguments]\n\n", name)

		if a.info.Summary != "" {
			fmt.Fprintln(a.errOut, a.info.Summary)
		}

		fmt.Fprintf(a.errOut, "\nCommands:\n\n")

		for _, command := range a.commands {
			fmt.Fprintf(a.errOut, "\t%s\t%s\n", command.name, command.summary)
		}

		printFlagDefaults(a.flags, true)
	}
}

func (a *MultiCommandApp) helpExecutor(ctx context.Context, arguments []string, out io.Writer) error {
	a.PrintHelp("help")

	return nil
}

func printCommandUsage(name string, summary string, flags Flags, out io.Writer) {
	fmt.Fprintf(out, "Usage: %s [arguments]\n\n", name)

	if summary != "" {
		fmt.Fprintln(out, summary)
	}

	printFlagDefaults(flags, false)
}

// printFlagDefaults wraps the writing of flag default values
func printFlagDefaults(flags Flags, asGlobal bool) {
	var buffer bytes.Buffer
	originalOut := flags.Output()

	flags.SetOutput(&buffer)
	flags.PrintDefaults()

	if buffer.Len() > 0 {
		// Only write a header if the printing of defaults actually wrote bytes
		switch asGlobal {
		case true:
			fmt.Fprintf(originalOut, "\nGlobal Options: (must be placed before <command>)\n\n")
		case false:
			fmt.Fprintf(originalOut, "\nOptions:\n\n")
		}

		// Write the buffered flag output
		buffer.WriteTo(originalOut)
	}

	// Restore the original output
	flags.SetOutput(originalOut)
}

func reflectFlagsUsage(flags Flags) *reflect.Value {
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
		return nil
	}

	return &usageReflect
}
