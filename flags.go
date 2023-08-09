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
	if flags == a.flags.Flags {
		return false
	}

	for _, command := range a.commands {
		if flags == command.flags.Flags {
			return false
		}
	}

	return true
}

// printFlagDefaults wraps the writing of flag default values
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
