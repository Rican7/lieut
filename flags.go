// Copyright © 2023 Trevor N. Suarez (Rican7)

package lieut

import (
	"flag"
	"fmt"
	"io"
	"reflect"
	"strings"
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

// visitAllFlagger defines an interface for visiting all set flags.
//
// This interface is specifically designed for compatibility with the Go
// standard library's flag package. Other techniques (such as reflection) are
// employed to support similar functionality in other flag packages.
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

// flagInfo normalizes flag data across different flag library implementations.
type flagInfo struct {
	name      string
	shorthand string
	usage     string
	defValue  string
	value     flag.Value
	typeName  string
}

// printFlagDefaults wraps the writing of flag default values.
func (a *app) printFlagDefaults(flags Flags) {
	// Unwrap our internal flagSet if necessary
	inner := flags
	if fs, ok := flags.(*flagSet); ok {
		inner = fs.Flags
	}

	var others []flagInfo
	var version *flagInfo
	var help *flagInfo

	// Visit all flags and categorize them
	visited := visitFlags(inner, func(f flagInfo) {
		switch f.name {
		case "help":
			help = &f
		case "version":
			version = &f
		default:
			others = append(others, f)
		}
	})

	if !visited {
		// Fallback to default printing if we can't visit
		flags.PrintDefaults()
		return
	}

	// Build the ordered list: others, then version, then help
	all := others
	if version != nil {
		all = append(all, *version)
	}
	if help != nil {
		all = append(all, *help)
	}

	if len(all) == 0 {
		return
	}

	// Determine dash prefix and check for shorthands
	dashPrefix := "--"
	hasShorthands := false
	if _, isStd := inner.(*flag.FlagSet); isStd {
		dashPrefix = "-"
	}
	for _, f := range all {
		if f.shorthand != "" {
			hasShorthands = true
			break
		}
	}

	// Calculate flag entries and their max length for alignment
	maxLen := 0
	formattedParts := make([]string, len(all))
	for i, f := range all {
		var sb strings.Builder

		// Shorthand column
		if hasShorthands {
			if f.shorthand != "" {
				fmt.Fprintf(&sb, "-%s, ", f.shorthand)
			} else {
				sb.WriteString("    ")
			}
		}

		// Flag name
		sb.WriteString(dashPrefix)
		sb.WriteString(f.name)

		// Type name
		if f.typeName != "" && f.typeName != "bool" {
			sb.WriteString(" ")
			sb.WriteString(f.typeName)
		}

		formattedParts[i] = sb.String()
		if len(formattedParts[i]) > maxLen {
			maxLen = len(formattedParts[i])
		}
	}

	// Print the output with tab alignment
	fmt.Fprintf(flags.Output(), "\nOptions:\n\n")
	for i, f := range all {
		fmt.Fprintf(flags.Output(), "\t%-[1]*s\t%s", maxLen, formattedParts[i], f.usage)
		if f.defValue != "" && f.defValue != "false" && f.defValue != "0" && f.defValue != `""` {
			def := f.defValue
			if f.typeName == "string" && !strings.HasPrefix(def, `"`) {
				def = fmt.Sprintf("%q", def)
			}
			fmt.Fprintf(flags.Output(), " (default %s)", def)
		}
		fmt.Fprintln(flags.Output())
	}
}

// visitFlags attempts to visit all flags in a generic way, supporting both
// the standard library and third-party libraries (via reflection).
func visitFlags(flags Flags, fn func(flagInfo)) bool {
	// 1. Try standard library visitor pattern
	if vf, ok := flags.(visitAllFlagger); ok {
		vf.VisitAll(func(f *flag.Flag) {
			typeName, usage := flag.UnquoteUsage(f)
			fn(flagInfo{
				name:     f.Name,
				usage:    usage,
				defValue: f.DefValue,
				value:    f.Value,
				typeName: typeName,
			})
		})
		return true
	}

	// 2. Try reflection for other implementations (like pflag)
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

	if !m.IsValid() || m.Type().NumIn() != 1 {
		return false
	}

	// Create a generic callback for the VisitAll method
	callbackType := m.Type().In(0)
	callback := reflect.MakeFunc(callbackType, func(args []reflect.Value) []reflect.Value {
		f := reflect.Indirect(args[0])

		// Helper to safely get a string field
		getStr := func(name string) string {
			field := f.FieldByName(name)
			if field.IsValid() && field.Kind() == reflect.String {
				return field.String()
			}
			return ""
		}

		info := flagInfo{
			name:      getStr("Name"),
			shorthand: getStr("Shorthand"),
			usage:     getStr("Usage"),
			defValue:  getStr("DefValue"),
		}

		if valField := f.FieldByName("Value"); valField.IsValid() {
			val := valField.Interface()
			if v, ok := val.(flag.Value); ok {
				info.value = v
			}

			// Try to get the type name (common in libraries like pflag)
			if tv, ok := val.(interface{ Type() string }); ok {
				info.typeName = tv.Type()
			}
		}

		if info.name != "" {
			fn(info)
		}

		return nil
	})

	m.Call([]reflect.Value{callback})
	return true
}
