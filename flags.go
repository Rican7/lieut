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
	//
	// NOTE: We have to use `reflect.DeepEqual`, because the interface values
	// could be non-comparable and could panic at runtime.
	if !reflect.DeepEqual(a.flags.Flags, flagSet.Flags) {
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
	setUsage(flagSet, a.PrintHelp)
}

// setUsage sets the `.Usage` field of the flags for a given command.
func (a *MultiCommandApp) setUsage(flagSet *flagSet, commandName string) {
	setUsage(flagSet, func() { a.PrintHelp(commandName) })
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

func setUsage(flagSet *flagSet, usageFunc func()) {
	switch flags := flagSet.Flags.(type) {
	case *flag.FlagSet:
		// If we're dealing with the standard library flags, just set the usage
		// function natively
		flags.Usage = usageFunc
	default:
		// Otherwise, we'll have to use reflection to work generically...
		//
		// TODO: Remove the use of reflection one we can use the type-system to
		// reliably detect types with specific fields, and set them.
		//
		// We CAN try and enforce a specific type of the flag set itself, but
		// then we'll only be able to be fully compatible with one flag package,
		// and none of the popular forks (like github.com/spf13/pflag).
		//
		// Until then, we'll HAVE to use reflection... :(
		//
		// See: https://github.com/golang/go/issues/48522
		usageReflect := reflectFlagsUsage(flagSet.Flags)
		if usageReflect == nil || !usageReflect.CanSet() {
			return
		}

		reflectFunc := reflect.ValueOf(usageFunc)
		usageReflect.Set(reflectFunc)
	}
}

func reflectFlagsUsage(flags Flags) *reflect.Value {
	flagsReflect := reflect.ValueOf(flags)
	flagsReflect = reflectElemUntil(flagsReflect, func(value reflect.Value) bool {
		return value.Kind() == reflect.Struct
	})

	if flagsReflect.Kind() != reflect.Struct {
		return nil
	}

	usageReflect := flagsReflect.FieldByName("Usage")
	usageFuncType := reflect.TypeOf(flag.Usage)

	if !usageReflect.IsValid() || !usageReflect.Type().AssignableTo(usageFuncType) {
		if embedded := findEmbeddedFlagsStruct(flags); embedded != nil {
			return reflectFlagsUsage(embedded)
		}

		return nil
	}

	return &usageReflect
}

func findEmbeddedFlagsStruct(flags Flags) Flags {
	flagsReflect := reflect.ValueOf(flags)
	flagsReflect = reflectElemUntil(flagsReflect, func(value reflect.Value) bool {
		return value.Kind() == reflect.Struct
	})

	flagsType := reflect.TypeOf((*Flags)(nil)).Elem()
	for i := 0; i < flagsReflect.NumField(); i++ {
		field := flagsReflect.Field(i)

		field = reflectElemUntil(field, func(value reflect.Value) bool {
			canElem := value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface
			isStructPointer := canElem && value.Elem().Kind() == reflect.Struct

			return isStructPointer &&
				value.CanInterface() &&
				value.Type().Implements(flagsType)
		})

		if field.IsValid() && field.CanInterface() && field.Type().Implements(flagsType) {
			return field.Interface().(Flags)
		}
	}

	return nil
}

func reflectElemUntil(value reflect.Value, until func(value reflect.Value) bool) reflect.Value {
	for !until(value) && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		value = value.Elem()
	}
	return value
}
