package lieut

import (
	"context"
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
