// Copyright © 2023 Trevor N. Suarez (Rican7)

// Package lieut provides mechanisms to standardize command line app execution.
//
// Lieut, short for lieutenant, or "second-in-command" to a commander.
//
// An opinionated, feature-limited, no external dependency, "micro-framework"
// for building command line applications in Go.
package lieut

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// DefaultCommandUsage defines the default usage string for commands.
	DefaultCommandUsage = "[arguments ...]"

	// DefaultParentCommandUsage defines the default usage string for commands
	// that have sub-commands.
	DefaultParentCommandUsage = "<command> [arguments ...]"
)

// Exit codes.
const (
	// ExitCodeSuccess is the exit code returned when an app runs successfully.
	ExitCodeSuccess = 0

	// ExitCodeError is the exit code returned when an app encounters an error.
	ExitCodeError = 1

	// ExitCodeUsageError is the exit code returned when an app encounters a
	// usage error, such as invalid flags or missing required arguments.
	ExitCodeUsageError = 2
)

// Executor is a functional interface that defines an executable command.
//
// It takes a context and arguments, and returns an error (if any occurred).
type Executor func(ctx context.Context, arguments []string) error

// CommandInfo describes information about a command.
type CommandInfo struct {
	Name    string
	Summary string
	Usage   string
}

type command struct {
	info CommandInfo
	Executor

	flags *flagSet // Flags for each command
}

// AppInfo describes information about an app.
type AppInfo struct {
	Name    string
	Summary string
	Usage   string
	Version string
}

// app is a runnable application configuration.
type app struct {
	info AppInfo

	flags *flagSet // Flags for the entire app (global)

	out    io.Writer
	errOut io.Writer

	init        func() error
	helpPrinter func() // Set per-run to display context-appropriate help
}

// SingleCommandApp is a runnable application that only has one command.
type SingleCommandApp struct {
	app

	exec Executor
}

// MultiCommandApp is a runnable application that has many commands.
type MultiCommandApp struct {
	app

	commands     map[string]command
	commandNames []string // Separate slice, to ensure consistent command order
}

// NewSingleCommandApp returns an initialized SingleCommandApp.
//
// The provided flags should have ContinueOnError ErrorHandling, or else flag
// parsing errors won't properly be displayed/handled.
func NewSingleCommandApp(info AppInfo, exec Executor, flags Flags, out io.Writer, errOut io.Writer) *SingleCommandApp {
	if info.Name == "" {
		info.Name = inferAppName()
	}

	if info.Usage == "" {
		info.Usage = DefaultCommandUsage
	}

	if flags == nil {
		flags = createDefaultFlags(info.Name)
	}

	flagSet := &flagSet{Flags: flags}

	app := &SingleCommandApp{
		app: app{
			info: info,

			flags: flagSet,

			out:    out,
			errOut: errOut,
		},

		exec: exec,
	}

	app.setupFlagSet(app.flags, true)

	return app
}

// NewMultiCommandApp returns an initialized MultiCommandApp.
//
// The provided flags are global/shared among the app's commands.
//
// The provided flags should have ContinueOnError ErrorHandling, or else flag
// parsing errors won't properly be displayed/handled.
func NewMultiCommandApp(info AppInfo, flags Flags, out io.Writer, errOut io.Writer) *MultiCommandApp {
	if info.Name == "" {
		info.Name = inferAppName()
	}

	if info.Usage == "" {
		info.Usage = DefaultParentCommandUsage
	}

	if flags == nil {
		flags = createDefaultFlags(info.Name)
	}

	flagSet := &flagSet{Flags: flags}

	app := &MultiCommandApp{
		app: app{
			info: info,

			flags: flagSet,

			out:    out,
			errOut: errOut,
		},

		commands: make(map[string]command),
	}

	app.setupFlagSet(app.flags, true)

	return app
}

// SetCommand sets a command for the given info, executor, and flags.
//
// It returns an error if the provided flags have already been used for another
// command (or for the globals).
//
// The provided flags should have ContinueOnError ErrorHandling, or else flag
// parsing errors won't properly be displayed/handled.
func (a *MultiCommandApp) SetCommand(info CommandInfo, exec Executor, flags Flags) error {
	if info.Usage == "" {
		info.Usage = DefaultCommandUsage
	}

	if !a.isUniqueFlagSet(flags) {
		return errors.New("provided flags are duplicate")
	}

	if flags == nil {
		flags = createDefaultFlags(info.Name)
	}

	flagSet := &flagSet{Flags: flags}

	a.setupFlagSet(flagSet, false)

	if _, hasCommand := a.commands[info.Name]; !hasCommand {
		a.commandNames = append(a.commandNames, info.Name)
	}

	a.commands[info.Name] = command{info: info, Executor: exec, flags: flagSet}

	return nil
}

// CommandNames returns the names of the set commands.
func (a *MultiCommandApp) CommandNames() []string {
	names := make([]string, len(a.commandNames))

	copy(names, a.commandNames)

	return names
}

// Run takes a context and arguments, runs the expected command, and returns an
// exit code.
//
// If the init function or command Executor returns a HelpRequestedError, the
// help message will be displayed and the returned exit code will be 0.
//
// If the init function or command Executor returns a StatusCodeError, then the
// returned exit code will match that of the value returned by
// StatusCodeError.StatusCode().
func (a *SingleCommandApp) Run(ctx context.Context, arguments []string) int {
	a.helpPrinter = a.PrintHelp

	if len(arguments) == 0 {
		arguments = os.Args[1:]
	}

	if err := a.flags.Parse(arguments); err != nil {
		a.PrintUsageError(err)
		return ExitCodeUsageError
	}

	if intercepted := a.intercept(a.flags); intercepted {
		return ExitCodeSuccess
	}

	if err := a.initialize(); err != nil {
		return a.handleError(err)
	}

	return a.execute(ctx, a.exec, a.flags.Args())
}

// Run takes a context and arguments, runs the expected command, and returns an
// exit code.
//
// If the init function or command Executor returns a HelpRequestedError, the
// help message will be displayed and the returned exit code will be 0.
//
// If the init function or command Executor returns a StatusCodeError, then the
// returned exit code will match that of the value returned by
// StatusCodeError.StatusCode().
func (a *MultiCommandApp) Run(ctx context.Context, arguments []string) int {
	if len(arguments) == 0 {
		arguments = os.Args[1:]
	}

	if len(a.commands) == 0 || len(arguments) == 0 {
		a.PrintHelp("")
		return ExitCodeUsageError
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
		a.PrintUsageError(commandName, err)
		return ExitCodeUsageError
	}

	if intercepted := a.intercept(flags, commandName); intercepted {
		return ExitCodeSuccess
	}

	if !hasCommand {
		return a.printUnknownCommand(commandName)
	}

	a.helpPrinter = func() { a.PrintHelp(commandName) }

	if err := a.initialize(); err != nil {
		return a.handleError(err)
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

// PrintHelp prints the help info to the app's error output.
//
// It's exposed so it can be called or assigned to a flag set's usage function.
func (a *SingleCommandApp) PrintHelp() {
	a.printFullUsage()
	fmt.Fprintln(a.errOut)
	a.printVersion(true)
}

// PrintHelp prints the help info to the app's error output.
//
// It's exposed so it can be called or assigned to a flag set's usage function.
func (a *MultiCommandApp) PrintHelp(commandName string) {
	a.printFullUsage(commandName)
	fmt.Fprintln(a.errOut)
	a.printVersion(true)
}

// PrintUsage prints the usage to the app's error output.
func (a *app) PrintUsage() {
	fmt.Fprintf(a.errOut, "Usage: %s %s\n", a.info.Name, a.info.Usage)
}

// PrintUsage prints the usage to the app's error output.
func (a *MultiCommandApp) PrintUsage(commandName string) {
	name := a.fullCommandName(commandName)
	command, hasCommand := a.commands[commandName]

	if !hasCommand {
		a.app.PrintUsage()
		return
	}

	fmt.Fprintf(a.errOut, "Usage: %s %s\n", name, command.info.Usage)
}

// PrintUsageError prints a standardized usage error to the app's error output.
func (a *SingleCommandApp) PrintUsageError(err error) {
	if err != nil && a.printError(err) {
		// Print a spacer line if an error was printed
		fmt.Fprintln(a.errOut)
	}

	a.PrintUsage()

	fmt.Fprintf(a.errOut, "\nRun '%s --help' for usage.\n", a.info.Name)
}

// PrintUsageError prints a standardized usage error to the app's error output.
func (a *MultiCommandApp) PrintUsageError(commandName string, err error) {
	name := a.fullCommandName(commandName)

	if err != nil && a.printError(err) {
		// Print a spacer line if an error was printed
		fmt.Fprintln(a.errOut)
	}

	a.PrintUsage(commandName)

	fmt.Fprintf(a.errOut, "\nRun '%s --help' for usage.\n", name)
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

	err := exec(ctx, arguments)
	if err != nil && !errors.Is(err, context.Canceled) {
		return a.handleError(err)
	}

	return ExitCodeSuccess
}

// printError takes an error, prints it with formatting, and then returns
// whether or not any actual error message was printed.
func (a *app) printError(err error) bool {
	msg := err.Error()

	if msg == "" {
		// Return false to denote that no error was printed
		return false
	}

	fmt.Fprintf(a.errOut, "Error: %s\n", msg)

	return true
}

func (a *app) handleError(err error) int {
	exitCode := ExitCodeError

	switch {
	case errors.Is(err, ErrHelpRequested):
		exitCode = ExitCodeUsageError

		if a.printError(err) {
			fmt.Fprintln(a.errOut)
		}

		if a.helpPrinter != nil {
			a.helpPrinter()
		}
	default:
		a.printError(err)
	}

	var statusErr StatusCodeError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode()
	}

	return exitCode
}

func (a *MultiCommandApp) fullCommandName(commandName string) string {
	name := a.info.Name
	command, hasCommand := a.commands[commandName]

	if hasCommand {
		name = fmt.Sprintf("%s %s", name, command.info.Name)
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

	return ExitCodeError
}

func (a *SingleCommandApp) printFullUsage() {
	a.PrintUsage()

	if a.info.Summary != "" {
		fmt.Fprintln(a.errOut)
		fmt.Fprintln(a.errOut, a.info.Summary)
	}

	a.printFlagDefaults(a.flags)
}

func (a *MultiCommandApp) printFullUsage(commandName string) {
	command, hasCommand := a.commands[commandName]

	switch {
	case hasCommand:
		a.PrintUsage(commandName)

		if command.info.Summary != "" {
			fmt.Fprintln(a.errOut)
			fmt.Fprintln(a.errOut, command.info.Summary)
		}

		a.printFlagDefaults(command.flags)
	default:
		a.PrintUsage("")

		if a.info.Summary != "" {
			fmt.Fprintln(a.errOut)
			fmt.Fprintln(a.errOut, a.info.Summary)
		}

		fmt.Fprintf(a.errOut, "\nCommands:\n\n")

		maxNameLength := 0
		for _, name := range a.commandNames {
			if len(name) > maxNameLength {
				maxNameLength = len(name)
			}
		}

		for _, name := range a.commandNames {
			command := a.commands[name]
			fmt.Fprintf(a.errOut, "\t%-[1]*s\t%s\n", maxNameLength, command.info.Name, command.info.Summary)
		}

		a.printFlagDefaults(a.flags)
	}
}

func inferAppName() string {
	basename := filepath.Base(os.Args[0])
	extension := filepath.Ext(basename)

	return strings.TrimSuffix(basename, extension)
}
