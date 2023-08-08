package lieut

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"testing"
)

var testAppInfo = AppInfo{
	Name:    "test",
	Summary: "A test",
	Usage:   "testing",
	Version: "vTest",
}

var testNoOpExecutor = func(ctx context.Context, arguments []string, out io.Writer) error {
	return nil
}

func TestNewSingleCommandApp(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	app := NewSingleCommandApp(testAppInfo, testNoOpExecutor, flagSet, os.Stdout, os.Stderr)

	if app == nil {
		t.Error("NewSingleCommandApp returned nil")
	}
}

func TestNewMultiCommandApp(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	app := NewMultiCommandApp(testAppInfo, flagSet, os.Stdout, os.Stderr)

	if app == nil {
		t.Error("NewMultiCommandApp returned nil")
	}
}

func TestMultiCommandApp_SetCommand(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	app := NewMultiCommandApp(testAppInfo, flagSet, os.Stdout, os.Stderr)

	for testName, testData := range map[string]struct {
		info  CommandInfo
		exec  Executor
		flags Flags
	}{
		"all": {
			info: CommandInfo{
				Name:    "test",
				Summary: "testing",
				Usage:   "test testing test",
			},
			exec:  testNoOpExecutor,
			flags: flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError),
		},
		"only info": {
			info: CommandInfo{
				Name:    "test",
				Summary: "testing",
				Usage:   "test testing test",
			},
		},
		"only exec": {
			exec: testNoOpExecutor,
		},
		"only flags": {
			flags: flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError),
		},
		"zero values": {},
	} {
		t.Run(testName, func(t *testing.T) {
			err := app.SetCommand(testData.info, testData.exec, testData.flags)
			if err != nil {
				t.Errorf("SetCommand returned error: %v", err)
			}
		})
	}
}

func TestMultiCommandApp_CommandNames(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	app := NewMultiCommandApp(testAppInfo, flagSet, os.Stdout, os.Stderr)

	if names := app.CommandNames(); len(names) > 0 {
		t.Errorf("CommandNames returned a non-empty slice %v", names)
	}

	app.SetCommand(CommandInfo{Name: "foo"}, nil, nil)
	app.SetCommand(CommandInfo{Name: "bar"}, nil, nil)

	names := app.CommandNames()
	sort.Strings(names)

	if names[0] != "bar" && names[1] != "foo" {
		t.Errorf("CommandNames returned an unexpected slice %v", names)
	}
}

func TestSingleCommandApp_PrintVersion(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	var buf bytes.Buffer

	app := NewSingleCommandApp(testAppInfo, testNoOpExecutor, flagSet, &buf, &buf)

	app.PrintVersion()

	got := buf.String()
	want := fmt.Sprintf("%s %s (%s/%s)\n", testAppInfo.Name, testAppInfo.Version, runtime.GOOS, runtime.GOARCH)

	if got != want {
		t.Errorf("app.PrintVersion gave %q, want %q", got, want)
	}
}

func TestSingleCommandApp_PrintVersion_NoVersionProvided(t *testing.T) {
	appInfo := testAppInfo
	appInfo.Version = ""

	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	var buf bytes.Buffer

	app := NewSingleCommandApp(appInfo, testNoOpExecutor, flagSet, &buf, &buf)

	app.PrintVersion()

	got := buf.String()
	want := fmt.Sprintf("%s (%s/%s)\n", testAppInfo.Name, runtime.GOOS, runtime.GOARCH)

	if got != want {
		t.Errorf("app.PrintVersion gave %q, want %q", got, want)
	}
}

func TestMultiCommandApp_PrintVersion(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	var buf bytes.Buffer

	app := NewMultiCommandApp(testAppInfo, flagSet, &buf, &buf)

	app.PrintVersion()

	got := buf.String()
	want := fmt.Sprintf("%s %s (%s/%s)\n", testAppInfo.Name, testAppInfo.Version, runtime.GOOS, runtime.GOARCH)

	if got != want {
		t.Errorf("app.PrintVersion gave %q, want %q", got, want)
	}
}

func TestMultiCommandApp_PrintVersion_NoVersionProvided(t *testing.T) {
	appInfo := testAppInfo
	appInfo.Version = ""

	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	var buf bytes.Buffer

	app := NewMultiCommandApp(appInfo, flagSet, &buf, &buf)

	app.PrintVersion()

	got := buf.String()
	want := fmt.Sprintf("%s (%s/%s)\n", testAppInfo.Name, runtime.GOOS, runtime.GOARCH)

	if got != want {
		t.Errorf("app.PrintVersion gave %q, want %q", got, want)
	}
}

func TestSingleCommandApp_PrintUsage(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	var buf bytes.Buffer

	app := NewSingleCommandApp(testAppInfo, testNoOpExecutor, flagSet, &buf, &buf)

	app.PrintUsage()

	got := buf.String()
	want := fmt.Sprintf("Usage: %s %s\n", testAppInfo.Name, testAppInfo.Usage)

	if got != want {
		t.Errorf("app.PrintUsage gave %q, want %q", got, want)
	}
}

func TestSingleCommandApp_PrintUsage_NoUsageProvided(t *testing.T) {
	appInfo := testAppInfo
	appInfo.Usage = ""

	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	var buf bytes.Buffer

	app := NewSingleCommandApp(appInfo, testNoOpExecutor, flagSet, &buf, &buf)

	app.PrintUsage()

	got := buf.String()
	want := fmt.Sprintf("Usage: %s %s\n", testAppInfo.Name, DefaultCommandUsage)

	if got != want {
		t.Errorf("app.PrintUsage gave %q, want %q", got, want)
	}
}
