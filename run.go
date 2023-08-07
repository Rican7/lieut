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
	SetOutput(output io.Writer)
}

type boolFlagger interface {
	BoolVar(p *bool, name string, value bool, usage string)
}

type boolShortFlagger interface {
	BoolVarP(p *bool, name string, shorthand string, value bool, usage string)
}

type lookupVarFlagger interface {
	Lookup(name string) *flag.Flag
	Var(value flag.Value, name string, usage string)
}

type visitAllFlagger interface {
	VisitAll(fn func(*flag.Flag))
}

type flagSet struct {
	Flags

	requestedHelp    bool
	requestedVersion bool
}

type command struct {
	name    string
	summary string
	Executor

	flags *flagSet // Flags for each command
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

	flags *flagSet // Flags for the entire app (global)

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
		flags = flag.NewFlagSet(info.Name, flag.ContinueOnError)
	}

	flagSet := &flagSet{Flags: flags}

	app := &SingleCommandApp{
		app: &app{
			info: info,

			flags: flagSet,

			out:    out,
			errOut: errOut,
		},

		exec: exec,
	}

	app.setupFlagSet(app.flags)
	app.setUsage(app.flags)

	return app
}

// NewMultiCommandApp returns an initialized MultiCommandApp.
//
// The provided flags are global/shared among the app's commands.
func NewMultiCommandApp(info AppInfo, flags Flags, out io.Writer, errOut io.Writer) *MultiCommandApp {
	if flags == nil {
		flags = flag.NewFlagSet(info.Name, flag.ContinueOnError)
	}

	flagSet := &flagSet{Flags: flags}

	app := &MultiCommandApp{
		app: &app{
			info: info,

			flags: flagSet,

			out:    out,
			errOut: errOut,
		},

		commands: make(map[string]command),
	}

	app.setupFlagSet(app.flags)
	app.setUsage(app.flags, "")

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
		flags = flag.NewFlagSet(name, flag.ContinueOnError)
	}

	flagSet := &flagSet{Flags: flags}

	a.setupFlagSet(flagSet)
	a.setUsage(flagSet, name)

	a.commands[name] = command{name, summary, exec, flagSet}

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
	if len(arguments) == 0 {
		arguments = os.Args[1:]
	}

	if err := a.flags.Parse(arguments); err != nil {
		return 1
	}

	if intercepted := a.intercept(a.flags); intercepted {
		return 0
	}

	if err := a.initialize(); err != nil {
		return a.printErr(err, false)
	}

	return a.execute(ctx, a.exec, a.flags.Args())
}

// Run takes a context and arguments, runs the expected command, and returns an
// exit code.
func (a *MultiCommandApp) Run(ctx context.Context, arguments []string) int {
	if len(arguments) == 0 {
		arguments = os.Args[1:]
	}

	if len(a.commands) == 0 || len(arguments) == 0 {
		a.PrintHelp("")
		return 2
	}

	flags := a.flags
	commandName := arguments[0]

	cmd, hasCommand := a.commands[commandName]
	if !hasCommand && commandName[0] != '-' {
		return a.printUnknownCommand(commandName)
	}

	if hasCommand {
		flags = cmd.flags
		arguments = arguments[1:]
	}

	if err := flags.Parse(arguments); err != nil {
		return 1
	}

	if intercepted := a.intercept(flags, commandName); intercepted {
		return 0
	}

	if !hasCommand {
		return a.printUnknownCommand(commandName)
	}

	if err := a.initialize(); err != nil {
		return a.printErr(err, false)
	}

	return a.execute(ctx, cmd.Executor, cmd.flags.Args())
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
	a.printCommandUsage(a.info.Name, a.info.Summary, a.flags)
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

// PrintUsageError prints a standardized usage error to the app's error output.
func (a *SingleCommandApp) PrintUsageError(err error) {
	name := a.info.Name

	fmt.Fprintf(a.errOut, "%s: %v\n", name, err)
	fmt.Fprintf(a.errOut, "Run '%s --help' for usage.\n", name)
}

// PrintUsageError prints a standardized usage error to the app's error output.
func (a *MultiCommandApp) PrintUsageError(commandName string, err error) {
	name := a.fullCommandName(commandName)

	fmt.Fprintf(a.errOut, "%s: %v\n", name, err)
	fmt.Fprintf(a.errOut, "Run '%s --help' for usage.\n", name)
}

func (a *app) intercept(flagSet *flagSet) bool {
	if flagSet.requestedVersion {
		a.PrintVersion()
		return true
	}

	return false
}

func (a *SingleCommandApp) intercept(flagSet *flagSet) bool {
	if flagSet.requestedHelp {
		a.PrintHelp()
		return true
	}

	return a.app.intercept(flagSet)
}

func (a *MultiCommandApp) intercept(flagSet *flagSet, commandName string) bool {
	if flagSet.requestedHelp {
		a.PrintHelp(commandName)
		return true
	}

	return a.app.intercept(flagSet)
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

func (a *app) setupFlagSet(flagSet *flagSet) {
	flagSet.SetOutput(a.errOut)

	helpDescription := "Display the help message"

	switch flags := flagSet.Flags.(type) {
	case boolShortFlagger:
		flags.BoolVarP(&flagSet.requestedHelp, "help", "h", flagSet.requestedHelp, helpDescription)
	case boolFlagger:
		flags.BoolVar(&flagSet.requestedHelp, "help", flagSet.requestedHelp, helpDescription)
	}

	if flags, ok := flagSet.Flags.(boolFlagger); ok {
		flags.BoolVar(&flagSet.requestedVersion, "version", flagSet.requestedVersion, "Display the application version")
	}

	// If the passed flags are not the app's global/shared flags
	if a.flags.Flags != flagSet.Flags {
		globalFlags, globalFlagsOk := a.flags.Flags.(visitAllFlagger)
		flags, flagsOk := flagSet.Flags.(lookupVarFlagger)

		if globalFlagsOk && flagsOk {
			// Loop through the globals and merge them into the specifics
			globalFlags.VisitAll(func(flag *flag.Flag) {
				// Don't override any existing flags (which causes panics...)
				if existing := flags.Lookup(flag.Name); existing == nil {
					flags.Var(flag.Value, flag.Name, flag.Usage)
				}
			})
		}
	}
}

// setUsage sets the `.Usage` field of the flags.
func (a *SingleCommandApp) setUsage(flagSet *flagSet) {
	usageReflect := reflectFlagsUsage(flagSet)
	if usageReflect == nil || !usageReflect.CanSet() {
		return
	}

	reflectFunc := reflect.ValueOf(a.PrintHelp)
	usageReflect.Set(reflectFunc)
}

// setUsage sets the `.Usage` field of the flags for a given command.
func (a *MultiCommandApp) setUsage(flagSet *flagSet, commandName string) {
	usageReflect := reflectFlagsUsage(flagSet)
	if usageReflect == nil || !usageReflect.CanSet() {
		return
	}

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

	if commandName != "" {
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

func (a *MultiCommandApp) printUnknownCommand(commandName string) int {
	a.PrintUsageError("", fmt.Errorf("unknown command '%s'", commandName))

	return 1
}

func (a *MultiCommandApp) printUsage(commandName string) {
	name := a.fullCommandName(commandName)
	command, hasCommand := a.commands[commandName]

	switch {
	case hasCommand:
		a.printCommandUsage(name, command.summary, command.flags)
	default:
		fmt.Fprintf(a.errOut, "Usage: %s <command> [arguments]\n\n", name)

		if a.info.Summary != "" {
			fmt.Fprintln(a.errOut, a.info.Summary)
		}

		fmt.Fprintf(a.errOut, "\nCommands:\n\n")

		for _, command := range a.commands {
			fmt.Fprintf(a.errOut, "\t%s\t%s\n", command.name, command.summary)
		}

		a.printFlagDefaults(a.flags)
	}
}

// printFlagDefaults wraps the writing of flag default values
func (a *app) printCommandUsage(name string, summary string, flags Flags) {
	fmt.Fprintf(a.errOut, "Usage: %s [arguments]\n\n", name)

	if summary != "" {
		fmt.Fprintln(a.errOut, summary)
	}

	a.printFlagDefaults(flags)
}

func (a *app) printFlagDefaults(flags Flags) {
	var buffer bytes.Buffer
	originalOut := a.errOut

	flags.SetOutput(&buffer)
	flags.PrintDefaults()

	if buffer.Len() > 0 {
		// Only write a header if the printing of defaults actually wrote bytes
		fmt.Fprintf(originalOut, "\nOptions:\n\n")

		// Write the buffered flag output
		buffer.WriteTo(originalOut)
	}

	// Restore the original output
	flags.SetOutput(originalOut)
}

func reflectFlagsUsage(flagSet *flagSet) *reflect.Value {
	// TODO: Remove the use of reflection one we can use the type-system to
	// reliably detect types with specific fields, and set them.
	//
	// We CAN try and enforce a specific type of the flag set itself, but then
	// we'll only be able to be fully compatible with one flag package, and noe
	// of the popular forks (like github.com/spf13/pflag).
	//
	// Until then, we'll HAVE to use reflection... :(
	flagsReflect := reflect.ValueOf(flagSet.Flags)
	if flagsReflect.IsValid() && flagsReflect.Kind() == reflect.Pointer {
		flagsReflect = flagsReflect.Elem()
	}

	usageReflect := flagsReflect.FieldByName("Usage")

	if !usageReflect.IsValid() || usageReflect.Kind() != reflect.Func {
		return nil
	}

	return &usageReflect
}
