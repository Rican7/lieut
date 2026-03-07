// Copyright © 2023 Trevor N. Suarez (Rican7)

package lieut

import (
	"bytes"
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
	typeName  string
}

// printFlagDefaults wraps the writing of flag default values.
func (a *app) printFlagDefaults(flags Flags) {
	// Unwrap our internal flagSet if necessary
	inner := flags
	if fs, ok := flags.(*flagSet); ok {
		inner = fs.Flags
	}

	// Visit and categorize flags for reordering
	userFlags, version, help, visited := categorizeFlagsByName(inner)

	if !visited {
		var buffer bytes.Buffer
		originalOut := flags.Output()

		flags.SetOutput(&buffer)
		flags.PrintDefaults()
		flags.SetOutput(originalOut)

		if buffer.Len() > 0 {
			fmt.Fprintf(originalOut, "\nOptions:\n\n")
			buffer.WriteTo(originalOut)
		}
		return
	}

	// Order entries: user flags first, then version, then help last
	all := userFlags
	if version != nil {
		all = append(all, *version)
	}
	if help != nil {
		all = append(all, *help)
	}

	if len(all) == 0 {
		return
	}

	// Determine dash prefix based on the flag implementation.
	// Standard library flag uses single-dash, while other implementations
	// (like spf13/pflag) use double-dash by convention.
	dashPrefix := "--"
	if _, isStd := inner.(*flag.FlagSet); isStd {
		dashPrefix = "-"
	}

	// Check if any flags have shorthands to decide on column layout
	hasShorthands := false
	for _, f := range all {
		if f.shorthand != "" {
			hasShorthands = true
			break
		}
	}

	// Format flag names and calculate max width for tab-stop alignment
	maxLen := 0
	formattedNames := make([]string, len(all))
	for i, f := range all {
		formattedNames[i] = formatFlagName(f, dashPrefix, hasShorthands)
		if len(formattedNames[i]) > maxLen {
			maxLen = len(formattedNames[i])
		}
	}

	// Print standardized, tab-aligned options
	out := flags.Output()
	fmt.Fprintf(out, "\nOptions:\n\n")
	for i, f := range all {
		fmt.Fprintf(out, "\t%-[1]*s\t%s", maxLen, formattedNames[i], f.usage)

		// Print default value if it's non-zero
		if f.defValue != "" && f.defValue != "false" && f.defValue != "0" && f.defValue != `""` {
			def := f.defValue
			if (f.typeName == "string" || f.typeName == "") && !strings.HasPrefix(def, `"`) {
				def = fmt.Sprintf("%q", def)
			}
			fmt.Fprintf(out, " (default %s)", def)
		}
		fmt.Fprintln(out)
	}
}

// categorizeFlagsByName visits all flags and separates them into user-defined
// flags, and the special version and help flags.
func categorizeFlagsByName(flags Flags) (userFlags []flagInfo, version, help *flagInfo, visited bool) {
	visited = visitFlags(flags, func(f flagInfo) {
		switch f.name {
		case "help":
			help = &f
		case "version":
			version = &f
		default:
			userFlags = append(userFlags, f)
		}
	})
	return
}

// formatFlagName formats the name portion of a flag for display, including
// shorthand, prefix, and type information.
func formatFlagName(f flagInfo, dashPrefix string, hasShorthands bool) string {
	var sb strings.Builder

	if hasShorthands {
		if f.shorthand != "" {
			fmt.Fprintf(&sb, "-%s, ", f.shorthand)
		} else {
			sb.WriteString("    ")
		}
	}

	sb.WriteString(dashPrefix)
	sb.WriteString(f.name)

	if f.typeName != "" && f.typeName != "bool" {
		fmt.Fprintf(&sb, " %s", f.typeName)
	}

	return sb.String()
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
				typeName: typeName,
			})
		})
		return true
	}

	// 2. Try reflection for other implementations (like pflag)
	m := reflect.ValueOf(flags).MethodByName("VisitAll")
	if !m.IsValid() || m.Type().NumIn() != 1 {
		return false
	}

	callbackType := m.Type().In(0)
	if callbackType.Kind() != reflect.Func {
		return false
	}

	callback := reflect.MakeFunc(callbackType, func(args []reflect.Value) []reflect.Value {
		f := reflect.Indirect(args[0])
		if f.Kind() != reflect.Struct {
			return nil
		}

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
			if tv, ok := valField.Interface().(interface{ Type() string }); ok {
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
