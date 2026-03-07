package integrations

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"
	"testing"

	"github.com/spf13/pflag"

	"github.com/Rican7/lieut"
)

var testAppInfo = lieut.AppInfo{
	Name:    "test",
	Summary: "A test",
	Usage:   "testing",
	Version: "vTest",
}

var testNoOpExecutor = func(ctx context.Context, arguments []string) error {
	return nil
}

func TestPFlag_WorkWithSingleCommandApps(t *testing.T) {
	flagSet := pflag.NewFlagSet("test", pflag.ExitOnError)
	out := io.Discard

	app := lieut.NewSingleCommandApp(testAppInfo, testNoOpExecutor, flagSet, out, out)

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

func TestPFlag__WorkWithMultiCommandApps(t *testing.T) {
	flagSet := pflag.NewFlagSet("test", pflag.ExitOnError)
	commandFlagSet := pflag.NewFlagSet("foo", pflag.ExitOnError)
	out := io.Discard

	app := lieut.NewMultiCommandApp(testAppInfo, flagSet, out, out)
	if app == nil {
		t.Fatal("NewMultiCommandApp returned nil")
	}

	err := app.SetCommand(lieut.CommandInfo{Name: "foo"}, nil, commandFlagSet)
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

func TestPFlag_FlagOrder(t *testing.T) {
	wantFormat := `Usage: test testing

A test

Options:

  -my-flag string
    	string My custom flag
  -z-flag string
    	string A flag at the end
  -version
    	Display the application version
  -help
    	Display the help message

test vTest (%s/%s)
`
	want := fmt.Sprintf(wantFormat, runtime.GOOS, runtime.GOARCH)

	flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flagSet.String("my-flag", "", "My custom flag")
	flagSet.String("z-flag", "", "A flag at the end")

	var buf bytes.Buffer
	app := lieut.NewSingleCommandApp(testAppInfo, testNoOpExecutor, flagSet, &buf, &buf)

	app.PrintHelp()

	got := buf.String()

	if got != want {
		t.Errorf("app.PrintHelp gave %q, want %q", got, want)
	}
}
