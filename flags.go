// Copyright © 2023 Trevor N. Suarez (Rican7)

package lieut

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"reflect"
)

// Flags defines an interface for command flags.
type Flags interface {
	Parse(arguments []string) error
	Args() []string
	PrintDefaults()
	Output() io.Writer
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

func createDefaultFlags(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}

// Parse wraps the inner flag Parse method, making sure that the error output is
// discarded/silenced.
func (f *flagSet) Parse(arguments []string) error {
	originalOut := f.Output()

	f.SetOutput(io.Discard)
	defer f.SetOutput(originalOut)

	return f.Flags.Parse(arguments)
}

func (a *MultiCommandApp) isUniqueFlagSet(flags Flags) bool {
	// NOTE: We have to use `reflect.DeepEqual`, because the interface values
	// could be non-comparable and could panic at runtime.
	if reflect.DeepEqual(flags, a.flags.Flags) {
		return false
	}

	for _, command := range a.commands {
		if reflect.DeepEqual(flags, command.flags.Flags) {
			return false
		}
	}

	return true
}

func (a *app) setupFlagSet(flagSet *flagSet, isRoot bool) {
	flagSet.SetOutput(a.errOut)

	helpDescription := "Display the help message"

	switch flags := flagSet.Flags.(type) {
	case boolShortFlagger:
		flags.BoolVarP(&flagSet.requestedHelp, "help", "h", flagSet.requestedHelp, helpDescription)
	case boolFlagger:
		flags.BoolVar(&flagSet.requestedHelp, "help", flagSet.requestedHelp, helpDescription)
	}

	// If the passed flags are the app's global/shared flags
	if isRoot {
		if flags, ok := flagSet.Flags.(boolFlagger); ok {
			flags.BoolVar(
				&flagSet.requestedVersion,
				"version",
				flagSet.requestedVersion,
				"Display the application version",
			)
		}
	} else {
		globalFlags, globalFlagsOk := a.flags.Flags.(visitAllFlagger)
		flags, flagsOk := flagSet.Flags.(lookupVarFlagger)

		if globalFlagsOk && flagsOk {
			// Loop through the globals and merge them into the specifics
			globalFlags.VisitAll(func(flag *flag.Flag) {
				// Don't override any existing flags (which causes panics...)
				if existing := flags.Lookup(flag.Name); existing == nil {
					// Don't merge the version flag, as it should only be
					// available on the root/global flag set
					if flag.Name != "version" {
						flags.Var(flag.Value, flag.Name, flag.Usage)
					}
				}
			})
		}
	}
}

// printFlagDefaults wraps the writing of flag default values
func (a *app) printFlagDefaults(flags Flags) {
	var buffer bytes.Buffer
	originalOut := flags.Output()

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
