package integrations

import (
	"bytes"
	"context"
	"io"
	"strings"
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
	flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flagSet.String("my-flag", "", "My custom flag")
	flagSet.String("z-flag", "", "A flag at the end")

	var buf bytes.Buffer
	app := lieut.NewSingleCommandApp(testAppInfo, testNoOpExecutor, flagSet, &buf, &buf)

	app.PrintHelp()

	output := buf.String()
	t.Logf("Output:\n%s", output)

	// Check order of flags in "Options:" section
	optionsIdx := strings.Index(output, "Options:")
	if optionsIdx == -1 {
		t.Fatal("Options section not found")
	}

	optionsSection := output[optionsIdx:]
	helpIdx := strings.Index(optionsSection, "-help")
	myFlagIdx := strings.Index(optionsSection, "-my-flag")
	versionIdx := strings.Index(optionsSection, "-version")
	zFlagIdx := strings.Index(optionsSection, "-z-flag")

	if helpIdx == -1 || myFlagIdx == -1 || versionIdx == -1 || zFlagIdx == -1 {
		t.Fatalf(
			"Flags not found: help=%d, my-flag=%d, version=%d, z-flag=%d",
			helpIdx,
			myFlagIdx,
			versionIdx,
			zFlagIdx,
		)
	}

	// We expect: myFlag < zFlag < version < help
	if !(myFlagIdx < zFlagIdx && zFlagIdx < versionIdx && versionIdx < helpIdx) {
		t.Errorf(
			"Unexpected flag order. Expected others < version < help. Got my-flag at %d, z-flag at %d, version at %d, help at %d",
			myFlagIdx,
			zFlagIdx,
			versionIdx,
			helpIdx,
		)
	}
}
