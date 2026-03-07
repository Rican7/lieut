package lieut

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"testing"
)

type bogusFlags string

func (b *bogusFlags) Parse(arguments []string) error {
	return nil
}

func (b *bogusFlags) Args() []string {
	return []string{}
}

func (b *bogusFlags) PrintDefaults() {
}

func (b *bogusFlags) Output() io.Writer {
	return nil
}

func (b *bogusFlags) SetOutput(output io.Writer) {
}

// Provide method that's used by "pflag" libraries (like github.com/spf13/pflag)
// to validate that it's at least called in the interface checks.
func (b *bogusFlags) BoolVarP(p *bool, name string, shorthand string, value bool, usage string) {
}

func TestBogusFlags_WorkWithSingleCommandApps(t *testing.T) {
	flagSet := bogusFlags("global")
	out := io.Discard

	app := NewSingleCommandApp(testAppInfo, testNoOpExecutor, &flagSet, out, out)

	if app == nil {
		t.Fatal("NewSingleCommandApp returned nil")
	}

	// Execute the many methods and make sure they don't panic
	app.PrintVersion()
	app.PrintUsage()
	app.PrintHelp()
	app.PrintUsageError(nil)
	app.Run(context.TODO(), nil)
}

func TestBogusFlags_WorkWithMultiCommandApps(t *testing.T) {
	flagSet := bogusFlags("global")
	commandFlagSet := bogusFlags("foo")
	out := io.Discard

	app := NewMultiCommandApp(testAppInfo, &flagSet, out, out)
	if app == nil {
		t.Fatal("NewMultiCommandApp returned nil")
	}

	err := app.SetCommand(CommandInfo{Name: "foo"}, nil, &commandFlagSet)
	if err != nil {
		t.Fatalf("SetCommand returned error: %v", err)
	}

	// Execute the many methods and make sure they don't panic
	app.PrintVersion()
	app.PrintUsage("")
	app.PrintUsage("foo")
	app.PrintHelp("")
	app.PrintHelp("foo")
	app.PrintUsageError("", nil)
	app.PrintUsageError("foo", nil)
	app.Run(context.TODO(), nil)
}

// verboseBogusFlags is a Flags implementation without VisitAll that produces
// output in PrintDefaults, used to test the fallback output path.
type verboseBogusFlags struct {
	out io.Writer
}

func (v *verboseBogusFlags) Parse(arguments []string) error { return nil }
func (v *verboseBogusFlags) Args() []string                 { return nil }
func (v *verboseBogusFlags) Output() io.Writer              { return v.out }
func (v *verboseBogusFlags) SetOutput(output io.Writer)     { v.out = output }
func (v *verboseBogusFlags) PrintDefaults() {
	fmt.Fprintln(v.out, "  -someflag\tA flag description")
}

// mockFlagValue implements an interface with a Type() method, similar to
// third-party flag library value types (like spf13/pflag).
type mockFlagValue string

func (m mockFlagValue) Type() string { return string(m) }

// mockReflectFlag mimics a third-party flag struct with exported fields, used
// to exercise the reflection-based visitor in visitFlags.
type mockReflectFlag struct {
	Name      string
	Shorthand string
	Usage     string
	DefValue  string
	Value     mockFlagValue
}

// reflectableFlags is a Flags implementation with a VisitAll method that uses a
// non-standard callback signature, exercising the reflection-based visitor path
// in visitFlags.
type reflectableFlags struct {
	flags []mockReflectFlag
	out   io.Writer
}

func (r *reflectableFlags) Parse(arguments []string) error { return nil }
func (r *reflectableFlags) Args() []string                 { return nil }
func (r *reflectableFlags) PrintDefaults()                 {}
func (r *reflectableFlags) Output() io.Writer              { return r.out }
func (r *reflectableFlags) SetOutput(output io.Writer)     { r.out = output }
func (r *reflectableFlags) VisitAll(fn func(*mockReflectFlag)) {
	for i := range r.flags {
		fn(&r.flags[i])
	}
}

// minimalReflectFlag is a flag struct with only a Name field, used to test the
// reflection visitor when expected fields are missing.
type minimalReflectFlag struct {
	Name string
}

// minimalReflectableFlags is a Flags implementation that visits flags with only
// a Name field, exercising the missing-field fallback in visitFlags.
type minimalReflectableFlags struct {
	flags []minimalReflectFlag
	out   io.Writer
}

func (m *minimalReflectableFlags) Parse(arguments []string) error { return nil }
func (m *minimalReflectableFlags) Args() []string                 { return nil }
func (m *minimalReflectableFlags) PrintDefaults()                 {}
func (m *minimalReflectableFlags) Output() io.Writer              { return m.out }
func (m *minimalReflectableFlags) SetOutput(output io.Writer)     { m.out = output }
func (m *minimalReflectableFlags) VisitAll(fn func(*minimalReflectFlag)) {
	for i := range m.flags {
		fn(&m.flags[i])
	}
}

// nonFuncVisitAllFlags has a VisitAll method with a non-function parameter,
// used to test the reflection guard against non-func callback types.
type nonFuncVisitAllFlags struct {
	bogusFlags
}

func (n *nonFuncVisitAllFlags) VisitAll(name string) {}

// stringVisitAllFlags has a VisitAll method that passes strings instead of
// structs, used to test the reflection guard against non-struct values.
type stringVisitAllFlags struct {
	bogusFlags
}

func (s *stringVisitAllFlags) VisitAll(fn func(string)) {
	fn("test-flag")
}

func TestPrintFlagDefaults(t *testing.T) {
	for testName, testData := range map[string]struct {
		flags Flags
		want  string
	}{
		"fallback with output": {
			flags: &verboseBogusFlags{},
			want:  "\nOptions:\n\n  -someflag\tA flag description\n",
		},
		"empty flagset": {
			flags: flag.NewFlagSet("empty", flag.ContinueOnError),
			want:  "",
		},
		"reflectable flags with shorthands": {
			flags: &reflectableFlags{
				flags: []mockReflectFlag{
					{Name: "output", Shorthand: "o", Usage: "Output file", Value: "string"},
					{Name: "verbose", Usage: "Enable verbose output", Value: "bool"},
					{Name: "help", Shorthand: "h", Usage: "Display the help message", Value: "bool"},
				},
			},
			want: "\nOptions:\n\n" +
				"\t-o, --output string\tOutput file\n" +
				"\t    --verbose      \tEnable verbose output\n" +
				"\t-h, --help         \tDisplay the help message\n",
		},
	} {
		t.Run(testName, func(t *testing.T) {
			var buf bytes.Buffer
			testData.flags.SetOutput(&buf)

			app := NewSingleCommandApp(testAppInfo, testNoOpExecutor, nil, io.Discard, io.Discard)
			app.printFlagDefaults(testData.flags)

			got := buf.String()
			if got != testData.want {
				t.Errorf("printFlagDefaults gave %q, want %q", got, testData.want)
			}
		})
	}
}

func TestFormatFlagName(t *testing.T) {
	for testName, testData := range map[string]struct {
		f             flagInfo
		dashPrefix    string
		hasShorthands bool
		want          string
	}{
		"simple": {
			f:          flagInfo{name: "verbose"},
			dashPrefix: "-",
			want:       "-verbose",
		},
		"with type": {
			f:          flagInfo{name: "output", typeName: "string"},
			dashPrefix: "--",
			want:       "--output string",
		},
		"bool type omitted": {
			f:          flagInfo{name: "verbose", typeName: "bool"},
			dashPrefix: "--",
			want:       "--verbose",
		},
		"with shorthand": {
			f:             flagInfo{name: "help", shorthand: "h"},
			dashPrefix:    "--",
			hasShorthands: true,
			want:          "-h, --help",
		},
		"without shorthand in shorthand layout": {
			f:             flagInfo{name: "version"},
			dashPrefix:    "--",
			hasShorthands: true,
			want:          "    --version",
		},
	} {
		t.Run(testName, func(t *testing.T) {
			got := formatFlagName(testData.f, testData.dashPrefix, testData.hasShorthands)
			if got != testData.want {
				t.Errorf("formatFlagName gave %q, want %q", got, testData.want)
			}
		})
	}
}

func TestVisitFlags(t *testing.T) {
	for testName, testData := range map[string]struct {
		flags      Flags
		wantResult bool
		wantFlags  []flagInfo
	}{
		"reflection": {
			flags: &reflectableFlags{
				flags: []mockReflectFlag{
					{Name: "verbose", Usage: "Enable verbose output", DefValue: "false", Value: "bool"},
					{Name: "output", Shorthand: "o", Usage: "Output file", Value: "string"},
				},
			},
			wantResult: true,
			wantFlags: []flagInfo{
				{name: "verbose", usage: "Enable verbose output", defValue: "false", typeName: "bool"},
				{name: "output", shorthand: "o", usage: "Output file", typeName: "string"},
			},
		},
		"missing fields": {
			flags: &minimalReflectableFlags{
				flags: []minimalReflectFlag{{Name: "test"}},
			},
			wantResult: true,
			wantFlags:  []flagInfo{{name: "test"}},
		},
		"non-func VisitAll": {
			flags:      &nonFuncVisitAllFlags{},
			wantResult: false,
		},
		"non-struct callback": {
			flags:      &stringVisitAllFlags{},
			wantResult: true,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			var visited []flagInfo
			result := visitFlags(testData.flags, func(f flagInfo) {
				visited = append(visited, f)
			})

			if result != testData.wantResult {
				t.Errorf("visitFlags returned %v, want %v", result, testData.wantResult)
			}

			if len(visited) != len(testData.wantFlags) {
				t.Fatalf("visitFlags visited %d flags, want %d", len(visited), len(testData.wantFlags))
			}

			for i, want := range testData.wantFlags {
				if visited[i] != want {
					t.Errorf("flag %d: got %+v, want %+v", i, visited[i], want)
				}
			}
		})
	}
}
