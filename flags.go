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

func (f *flagSet) VisitAll(fn func(*flag.Flag)) {
	if vf, ok := f.Flags.(visitAllFlagger); ok {
		vf.VisitAll(fn)
	}
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
	if fs, ok := flags.(*flagSet); ok {
		flags = fs.Flags
	}

	var buffer bytes.Buffer
	originalOut := flags.Output()

	flags.SetOutput(&buffer)

	// Function to visit flags generically using reflection if necessary
	visitAll := func(fn func(name, usage, defValue string, value flag.Value)) bool {
		// Try standard interface first
		if vf, ok := flags.(visitAllFlagger); ok {
			vf.VisitAll(func(f *flag.Flag) {
				fn(f.Name, f.Usage, f.DefValue, f.Value)
			})
			return true
		}

		// Use reflection for other implementations (like pflag)
		v := reflect.ValueOf(flags)
		m := v.MethodByName("VisitAll")
		if !m.IsValid() {
			// Try the underlying value if it's an interface or pointer
			for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
				v = v.Elem()
				m = v.MethodByName("VisitAll")
				if m.IsValid() {
					break
				}
			}
		}

		if !m.IsValid() {
			return false
		}

		mType := m.Type()
		if mType.NumIn() != 1 {
			return false
		}

		callbackType := mType.In(0)
		if callbackType.Kind() != reflect.Func || callbackType.NumIn() != 1 {
			return false
		}

		callback := reflect.MakeFunc(callbackType, func(args []reflect.Value) []reflect.Value {
			f := args[0]
			for f.Kind() == reflect.Pointer || f.Kind() == reflect.Interface {
				f = f.Elem()
			}

			// Helper to get string field if it exists
			getString := func(name string) string {
				field := f.FieldByName(name)
				if field.IsValid() && field.Kind() == reflect.String {
					return field.String()
				}
				return ""
			}

			name := getString("Name")
			usage := getString("Usage")
			defValue := getString("DefValue")

			// Get Value field
			var val flag.Value
			valField := f.FieldByName("Value")
			if valField.IsValid() {
				if v, ok := valField.Interface().(flag.Value); ok {
					val = v
				}
			}

			if name != "" {
				fn(name, usage, defValue, val)
			}

			return nil
		})

		m.Call([]reflect.Value{callback})
		return true
	}

	type flagInfo struct {
		name     string
		usage    string
		defValue string
		value    flag.Value
	}

	var others []flagInfo
	var version *flagInfo
	var help []flagInfo

	visited := visitAll(func(name, usage, defValue string, value flag.Value) {
		info := flagInfo{name, usage, defValue, value}
		switch name {
		case "help", "h":
			help = append(help, info)
		case "version":
			version = &info
		default:
			others = append(others, info)
		}
	})
	if visited {
		// Helper to print a group of flags using a temporary FlagSet to
		// maintain the standard library's formatting
		printGroup := func(group []flagInfo) {
			if len(group) == 0 {
				return
			}

			temp := flag.NewFlagSet("", flag.ContinueOnError)
			temp.SetOutput(&buffer)

			for _, f := range group {
				temp.Var(f.value, f.name, f.usage)
				temp.Lookup(f.name).DefValue = f.defValue
			}

			temp.PrintDefaults()
		}

		printGroup(others)
		if version != nil {
			printGroup([]flagInfo{*version})
		}
		printGroup(help)

	} else {
		// Fallback to default printing if we can't visit
		flags.PrintDefaults()
	}

	if buffer.Len() > 0 {
		// Only write a header if the printing of defaults actually wrote bytes
		fmt.Fprintf(originalOut, "\nOptions:\n\n")

		// Write the buffered flag output
		buffer.WriteTo(originalOut)
	}

	// Restore the original output
	flags.SetOutput(originalOut)
}
