package lieut

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"runtime"
	"testing"
	"time"
)

var testAppInfo = AppInfo{
	Name:    "test",
	Summary: "A test",
	Usage:   "testing",
	Version: "vTest",
}

var testNoOpExecutor = func(ctx context.Context, arguments []string) error {
	return nil
}

func TestMain(m *testing.M) {
	originalOSArgs := os.Args[:]
	defer func() {
		// Put back the original args... to not mess with global state
		os.Args = originalOSArgs
	}()

	// Parse the flags, as the testing package needs them
	flag.Parse()

	// Set the args to just the executable name
	// (removing passed flags to the test executable)
	os.Args = os.Args[0:1]

	os.Exit(m.Run())
}

func TestNewSingleCommandApp(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	app := NewSingleCommandApp(testAppInfo, testNoOpExecutor, flagSet, os.Stdout, os.Stderr)

	if app == nil {
		t.Error("NewSingleCommandApp returned nil")
	}
}

func TestNewSingleCommandApp_ZeroValues(t *testing.T) {
	app := NewSingleCommandApp(AppInfo{}, nil, nil, nil, nil)

	if app == nil {
		t.Fatal("NewSingleCommandApp returned nil")
	}

	if inferredName := inferAppName(); app.info.Name != inferredName {
		t.Errorf("NewSingleCommandApp with no given name gave %q name, wanted %q", app.info.Name, inferredName)
	}

	if app.info.Usage != DefaultCommandUsage {
		t.Errorf(
			"NewSingleCommandApp with no given usage gave %q usage, wanted %q",
			app.info.Usage,
			DefaultCommandUsage,
		)
	}

	if app.flags.Flags == nil {
		t.Errorf("NewSingleCommandApp with no given flags had nil flags")
	}
}

func TestNewMultiCommandApp(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	app := NewMultiCommandApp(testAppInfo, flagSet, os.Stdout, os.Stderr)

	if app == nil {
		t.Error("NewMultiCommandApp returned nil")
	}
}

func TestNewMultiCommandApp_ZeroValues(t *testing.T) {
	app := NewMultiCommandApp(AppInfo{}, nil, nil, nil)

	if app == nil {
		t.Fatal("NewMultiCommandApp returned nil")
	}

	if inferredName := inferAppName(); app.info.Name != inferredName {
		t.Errorf("NewMultiCommandApp with no given name gave %q name, wanted %q", app.info.Name, inferredName)
	}

	if app.info.Usage != DefaultParentCommandUsage {
		t.Errorf(
			"NewMultiCommandApp with no given usage gave %q usage, wanted %q",
			app.info.Usage,
			DefaultParentCommandUsage,
		)
	}

	if app.flags.Flags == nil {
		t.Errorf("NewMultiCommandApp with no given flags had nil flags")
	}
}

func TestMultiCommandApp_SetCommand(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)

	app := NewMultiCommandApp(testAppInfo, flagSet, os.Stdout, os.Stderr)

	includedCommandFlagSet := flag.NewFlagSet("included", flag.ExitOnError)
	err := app.SetCommand(CommandInfo{Name: "included"}, nil, includedCommandFlagSet)
	if err != nil {
		t.Fatalf("SetCommand returned error: %v", err)
	}

	for testName, testData := range map[string]struct {
		info    CommandInfo
		exec    Executor
		flags   Flags
		wantErr bool
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
		"duplicate flags from globals": {
			flags:   flagSet,
			wantErr: true,
		},
		"duplicate flags from other command": {
			flags:   includedCommandFlagSet,
			wantErr: true,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			err := app.SetCommand(testData.info, testData.exec, testData.flags)
			if err != nil && !testData.wantErr {
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

	_ = app.SetCommand(CommandInfo{Name: "foo"}, nil, nil)
	_ = app.SetCommand(CommandInfo{Name: "bar"}, nil, nil)

	names := app.CommandNames()

	if names[0] != "foo" || names[1] != "bar" {
		t.Errorf("CommandNames returned an unexpected slice %v", names)
	}
}

func TestSingleCommandApp_PrintVersion(t *testing.T) {
	for testName, testData := range map[string]struct {
		version string
		want    string
	}{
		"specified": {
			version: "vTest",
			want:    fmt.Sprintf("%s vTest (%s/%s)\n", testAppInfo.Name, runtime.GOOS, runtime.GOARCH),
		},
		"no version string": {
			version: "",
			want:    fmt.Sprintf("%s (%s/%s)\n", testAppInfo.Name, runtime.GOOS, runtime.GOARCH),
		},
	} {
		t.Run(testName, func(t *testing.T) {
			appInfo := testAppInfo
			appInfo.Version = testData.version
			var buf bytes.Buffer

			flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
			app := NewSingleCommandApp(appInfo, testNoOpExecutor, flagSet, &buf, &buf)

			app.PrintVersion()

			got := buf.String()

			if got != testData.want {
				t.Errorf("app.PrintVersion gave %q, want %q", got, testData.want)
			}
		})
	}
}

func TestMultiCommandApp_PrintVersion(t *testing.T) {
	for testName, testData := range map[string]struct {
		version string
		want    string
	}{
		"specified": {
			version: "vTest",
			want:    fmt.Sprintf("%s vTest (%s/%s)\n", testAppInfo.Name, runtime.GOOS, runtime.GOARCH),
		},
		"no version string": {
			version: "",
			want:    fmt.Sprintf("%s (%s/%s)\n", testAppInfo.Name, runtime.GOOS, runtime.GOARCH),
		},
	} {
		t.Run(testName, func(t *testing.T) {
			appInfo := testAppInfo
			appInfo.Version = testData.version
			var buf bytes.Buffer

			flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
			app := NewMultiCommandApp(appInfo, flagSet, &buf, &buf)

			app.PrintVersion()

			got := buf.String()

			if got != testData.want {
				t.Errorf("app.PrintVersion gave %q, want %q", got, testData.want)
			}
		})
	}
}

func TestSingleCommandApp_PrintHelp(t *testing.T) {
	wantFormat := `Usage: test testing

A test

Options:

  -help
    	Display the help message
  -testflag string
    	A test flag (default "testval")
  -version
    	Display the application version

test vTest (%s/%s)
`
	want := fmt.Sprintf(wantFormat, runtime.GOOS, runtime.GOARCH)

	var buf bytes.Buffer

	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
	flagSet.String("testflag", "testval", "A test flag")

	app := NewSingleCommandApp(testAppInfo, testNoOpExecutor, flagSet, &buf, &buf)

	app.PrintHelp()

	got := buf.String()

	if got != want {
		t.Errorf("app.PrintHelp gave %q, want %q", got, want)
	}
}

func TestMultiCommandApp_PrintHelp(t *testing.T) {
	wantFormat := `Usage: test testing

A test

Commands:

	testcommand	A test command

Options:

  -help
    	Display the help message
  -testflag string
    	A test flag (default "testval")
  -version
    	Display the application version

test vTest (%s/%s)
`
	want := fmt.Sprintf(wantFormat, runtime.GOOS, runtime.GOARCH)

	var buf bytes.Buffer

	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
	flagSet.String("testflag", "testval", "A test flag")

	app := NewMultiCommandApp(testAppInfo, flagSet, &buf, &buf)

	testCommandInfo := CommandInfo{
		Name:    "testcommand",
		Summary: "A test command",
		Usage:   "args here...",
	}

	commandFlagSet := flag.NewFlagSet(testCommandInfo.Name, flag.ExitOnError)
	commandFlagSet.Int("testcommandflag", 5, "A test command flag")

	err := app.SetCommand(testCommandInfo, testNoOpExecutor, commandFlagSet)
	if err != nil {
		t.Fatalf("SetCommand returned error: %v", err)
	}

	app.PrintHelp("")

	got := buf.String()

	if got != want {
		t.Errorf("app.PrintHelp gave %q, want %q", got, want)
	}
}

func TestMultiCommandApp_PrintHelp_Command(t *testing.T) {
	wantFormat := `Usage: test testcommand args here...

A test command

Options:

  -help
    	Display the help message
  -testcommandflag int
    	A test command flag (default 5)
  -testflag string
    	A test flag (default "testval")

test vTest (%s/%s)
`
	want := fmt.Sprintf(wantFormat, runtime.GOOS, runtime.GOARCH)

	var buf bytes.Buffer

	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
	flagSet.String("testflag", "testval", "A test flag")

	app := NewMultiCommandApp(testAppInfo, flagSet, &buf, &buf)

	testCommandInfo := CommandInfo{
		Name:    "testcommand",
		Summary: "A test command",
		Usage:   "args here...",
	}

	commandFlagSet := flag.NewFlagSet(testCommandInfo.Name, flag.ExitOnError)
	commandFlagSet.Int("testcommandflag", 5, "A test command flag")

	err := app.SetCommand(testCommandInfo, testNoOpExecutor, commandFlagSet)
	if err != nil {
		t.Fatalf("SetCommand returned error: %v", err)
	}

	app.PrintHelp("testcommand")

	got := buf.String()

	if got != want {
		t.Errorf("app.PrintHelp gave %q, want %q", got, want)
	}
}

func TestMultiCommandApp_PrintHelp_AlignmentAndOrder(t *testing.T) {
	wantFormat := `Usage: test testing

A test

Commands:

	a                  	Short summary
	longer             	Another summary
	much-longer-command	Yet another summary
	zzz                	Last summary

Options:

  -help
    	Display the help message
  -version
    	Display the application version

test vTest (%s/%s)
`
	want := fmt.Sprintf(wantFormat, runtime.GOOS, runtime.GOARCH)

	var buf bytes.Buffer

	app := NewMultiCommandApp(testAppInfo, nil, &buf, &buf)

	// Add in a specific order to test preservation
	_ = app.SetCommand(CommandInfo{Name: "a", Summary: "Short summary"}, testNoOpExecutor, nil)
	_ = app.SetCommand(CommandInfo{Name: "longer", Summary: "Another summary"}, testNoOpExecutor, nil)
	_ = app.SetCommand(CommandInfo{Name: "much-longer-command", Summary: "Yet another summary"}, testNoOpExecutor, nil)
	_ = app.SetCommand(CommandInfo{Name: "zzz", Summary: "Last summary"}, testNoOpExecutor, nil)

	app.PrintHelp("")

	got := buf.String()

	if got != want {
		t.Errorf("app.PrintHelp alignment/order gave %q, want %q", got, want)
	}
}

func TestMultiCommandApp_CommandOrderIsConsistent(t *testing.T) {
	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))

	for i := 0; i < 1000; i++ {
		app := NewMultiCommandApp(testAppInfo, nil, io.Discard, io.Discard)

		// Create a set of names and shuffle them
		names := []string{"a", "b", "c", "d", "e", "f", "g"}
		r.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })

		for _, name := range names {
			_ = app.SetCommand(CommandInfo{Name: name}, testNoOpExecutor, nil)
		}

		// Test that re-setting a command doesn't change the order
		updateName := names[r.Intn(len(names))]
		_ = app.SetCommand(CommandInfo{Name: updateName, Summary: "Updated"}, testNoOpExecutor, nil)

		got := app.CommandNames()

		if len(got) != len(names) {
			t.Fatalf("iteration %d (seed %d): got %d names, wanted %d", i, seed, len(got), len(names))
		}

		for j, name := range names {
			if got[j] != name {
				t.Errorf(
					"iteration %d (seed %d): CommandNames() gave %v at index %d, wanted %v",
					i,
					seed,
					got[j],
					j,
					name,
				)
			}
		}
	}
}

func TestSingleCommandApp_PrintUsage(t *testing.T) {
	for testName, testData := range map[string]struct {
		usage string
		want  string
	}{
		"specified": {
			usage: "testing [test]",
			want:  fmt.Sprintf("Usage: %s testing [test]\n", testAppInfo.Name),
		},
		"no usage string": {
			usage: "",
			want:  fmt.Sprintf("Usage: %s %s\n", testAppInfo.Name, DefaultCommandUsage),
		},
	} {
		t.Run(testName, func(t *testing.T) {
			appInfo := testAppInfo
			appInfo.Usage = testData.usage
			var buf bytes.Buffer

			flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
			app := NewSingleCommandApp(appInfo, testNoOpExecutor, flagSet, &buf, &buf)

			app.PrintUsage()

			got := buf.String()

			if got != testData.want {
				t.Errorf("app.PrintUsage gave %q, want %q", got, testData.want)
			}
		})
	}
}

func TestMultiCommandApp_PrintUsage(t *testing.T) {
	testCommandInfo := CommandInfo{
		Name:    "test",
		Summary: "testing",
	}

	for testName, testData := range map[string]struct {
		appUsage     string
		commandUsage string
		forCommand   string
		want         string
	}{
		"specified app and command usage, for command": {
			appUsage:     "testing [test]",
			commandUsage: "test [opts]",
			forCommand:   testCommandInfo.Name,
			want:         fmt.Sprintf("Usage: %s %s test [opts]\n", testAppInfo.Name, testCommandInfo.Name),
		},
		"specified app usage, no command usage, for command": {
			appUsage:     "testing [test]",
			commandUsage: "",
			forCommand:   testCommandInfo.Name,
			want:         fmt.Sprintf("Usage: %s %s %s\n", testAppInfo.Name, testCommandInfo.Name, DefaultCommandUsage),
		},
		"no app usage, specified command usage, for command": {
			appUsage:     "",
			commandUsage: "test [opts]",
			forCommand:   testCommandInfo.Name,
			want:         fmt.Sprintf("Usage: %s %s test [opts]\n", testAppInfo.Name, testCommandInfo.Name),
		},
		"no app or command usage, for command": {
			appUsage:     "",
			commandUsage: "",
			forCommand:   testCommandInfo.Name,
			want:         fmt.Sprintf("Usage: %s %s %s\n", testAppInfo.Name, testCommandInfo.Name, DefaultCommandUsage),
		},
		"specified app and command usage, no command": {
			appUsage:     "testing [test]",
			commandUsage: "test [opts]",
			forCommand:   "",
			want:         fmt.Sprintf("Usage: %s testing [test]\n", testAppInfo.Name),
		},
		"specified app usage, no command usage, no command": {
			appUsage:     "testing [test]",
			commandUsage: "",
			forCommand:   "",
			want:         fmt.Sprintf("Usage: %s testing [test]\n", testAppInfo.Name),
		},
		"no app usage, specified command usage, no command": {
			appUsage:     "",
			commandUsage: "test [opts]",
			forCommand:   "",
			want:         fmt.Sprintf("Usage: %s %s\n", testAppInfo.Name, DefaultParentCommandUsage),
		},
		"no app or command usage, no command": {
			appUsage:     "",
			commandUsage: "",
			forCommand:   "",
			want:         fmt.Sprintf("Usage: %s %s\n", testAppInfo.Name, DefaultParentCommandUsage),
		},
	} {
		t.Run(testName, func(t *testing.T) {
			appInfo := testAppInfo
			appInfo.Usage = testData.appUsage
			var buf bytes.Buffer

			flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
			app := NewMultiCommandApp(appInfo, flagSet, &buf, &buf)

			testCommandInfo.Usage = testData.commandUsage
			commandFlagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
			err := app.SetCommand(testCommandInfo, testNoOpExecutor, commandFlagSet)
			if err != nil {
				t.Errorf("SetCommand returned error: %v", err)
			}

			app.PrintUsage(testData.forCommand)

			got := buf.String()

			if got != testData.want {
				t.Errorf("app.PrintUsage gave %q, want %q", got, testData.want)
			}
		})
	}
}

func TestSingleCommandApp_PrintUsageError(t *testing.T) {
	var buf bytes.Buffer

	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
	app := NewSingleCommandApp(testAppInfo, testNoOpExecutor, flagSet, &buf, &buf)

	usageErr := errors.New("test usage error")
	want := `Error: test usage error

Usage: test testing

Run 'test --help' for usage.
`

	app.PrintUsageError(usageErr)

	got := buf.String()

	if got != want {
		t.Errorf("app.PrintUsageError gave %q, want %q", got, want)
	}
}

func TestMultiCommandApp_PrintUsageError(t *testing.T) {
	var buf bytes.Buffer

	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
	app := NewMultiCommandApp(testAppInfo, flagSet, &buf, &buf)

	usageErr := errors.New("test usage error")
	want := `Error: test usage error

Usage: test testing

Run 'test --help' for usage.
`

	app.PrintUsageError("", usageErr)

	got := buf.String()

	if got != want {
		t.Errorf("app.PrintUsageError gave %q, want %q", got, want)
	}
}

func TestMultiCommandApp_PrintUsageError_Command(t *testing.T) {
	var buf bytes.Buffer

	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
	app := NewMultiCommandApp(testAppInfo, flagSet, &buf, &buf)

	testCommandInfo := CommandInfo{Name: "testcommand"}
	_ = app.SetCommand(testCommandInfo, nil, nil)

	usageErr := errors.New("test usage error")
	want := `Error: test usage error

Usage: test testcommand [arguments ...]

Run 'test testcommand --help' for usage.
`

	app.PrintUsageError(testCommandInfo.Name, usageErr)

	got := buf.String()

	if got != want {
		t.Errorf("app.PrintUsageError gave %q, want %q", got, want)
	}
}

func TestSingleCommandApp_Run(t *testing.T) {
	var executorCapture struct {
		ctx       context.Context
		arguments []string
	}

	executor := func(ctx context.Context, arguments []string) error {
		executorCapture.ctx = ctx
		executorCapture.arguments = arguments

		return nil
	}

	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
	out := io.Discard

	app := NewSingleCommandApp(testAppInfo, executor, flagSet, out, out)

	ctxTestKey := struct{ k string }{k: "test-key-for-testing"}
	ctxTestVal := "test context val"
	ctx := context.WithValue(context.TODO(), ctxTestKey, ctxTestVal)
	args := []string{"testarg1", "testarg2"}
	wantedExitCode := 0

	initRan := false
	initFn := func() error {
		initRan = true
		return nil
	}

	app.OnInit(initFn)

	exitCode := app.Run(ctx, args)

	if exitCode != wantedExitCode {
		t.Errorf("app.Run gave %v, wanted %v", exitCode, wantedExitCode)
	}

	if !initRan {
		t.Error("app.Run didn't run init function")
	}

	if executorCapture.ctx.Value(ctxTestKey) != ctxTestVal {
		t.Errorf("app.Run executor gave %q, wanted %q", executorCapture.ctx.Value(ctxTestKey), ctxTestVal)
	}

	if executorCapture.arguments[0] != args[0] && executorCapture.arguments[1] != args[1] {
		t.Errorf("app.Run executor gave %q, wanted %q", executorCapture.arguments, args)
	}
}

func TestSingleCommandApp_Run_EmptyArgsProvided(t *testing.T) {
	var capturedArgs []string

	executor := func(ctx context.Context, arguments []string) error {
		capturedArgs = arguments
		return nil
	}

	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
	out := io.Discard

	app := NewSingleCommandApp(testAppInfo, executor, flagSet, out, out)

	originalOSArgs := os.Args[:]
	defer func() {
		// Put back the original args... to not mess with global state
		os.Args = originalOSArgs
	}()

	os.Args = []string{testAppInfo.Name, "arg"}
	expectedArgs := os.Args[1:]

	if exitCode := app.Run(context.TODO(), nil); exitCode != 0 {
		t.Errorf("app.Run gave non-zero exit code %v", exitCode)
	}

	if capturedArgs[0] != expectedArgs[0] {
		t.Errorf("app.Run executor gave args %q, wanted %q", capturedArgs, expectedArgs)
	}
}

func TestSingleCommandApp_Run_AltPaths(t *testing.T) {
	singleHelpOut := fmt.Sprintf(`Usage: test testing

A test

Options:

  -help
    	Display the help message
  -version
    	Display the application version

test vTest (%s/%s)
`, runtime.GOOS, runtime.GOARCH)

	for testName, testData := range map[string]struct {
		exec  Executor
		init  func() error
		flags Flags

		args []string

		wantedExitCode int
		wantedOut      string
		wantedErrOut   string
	}{
		"version requested": {
			args: []string{"--version"},

			wantedExitCode: 0,
			wantedOut:      fmt.Sprintf("test vTest (%s/%s)\n", runtime.GOOS, runtime.GOARCH),
			wantedErrOut:   "",
		},
		"help requested": {
			args: []string{"--help"},

			wantedExitCode: 0,
			wantedOut:      "",
			wantedErrOut: fmt.Sprintf(`Usage: test testing

A test

Options:

  -help
    	Display the help message
  -version
    	Display the application version

test vTest (%s/%s)
`, runtime.GOOS, runtime.GOARCH),
		},
		"args contain non-defined flags": {
			flags: flag.NewFlagSet("test", flag.ContinueOnError),
			args:  []string{"--non-existent-flag=val"},

			wantedExitCode: 2,
			wantedOut:      "",
			wantedErrOut: `Error: flag provided but not defined: -non-existent-flag

Usage: test testing

Run 'test --help' for usage.
`,
		},
		"initialize returns error": {
			init: func() error {
				return errors.New("test init error")
			},

			args: []string{"test"},

			wantedExitCode: 1,
			wantedOut:      "",
			wantedErrOut:   "Error: test init error\n",
		},
		"initialize returns status code error": {
			init: func() error {
				return ErrWithStatusCode(errors.New("test init error"), 101)
			},

			args: []string{"test"},

			wantedExitCode: 101,
			wantedOut:      "",
			wantedErrOut:   "Error: test init error\n",
		},
		"initialize returns status code empty error": {
			init: func() error {
				return ErrWithStatusCode(errors.New(""), 101)
			},

			args: []string{"test"},

			wantedExitCode: 101,
			wantedOut:      "",
			wantedErrOut:   "",
		},
		"execute returns error": {
			exec: func(ctx context.Context, arguments []string) error {
				return errors.New("test exec error")
			},

			args: []string{"test"},

			wantedExitCode: 1,
			wantedOut:      "",
			wantedErrOut:   "Error: test exec error\n",
		},
		"execute returns status code error": {
			exec: func(ctx context.Context, arguments []string) error {
				return ErrWithStatusCode(errors.New("test exec error"), 217)
			},

			args: []string{"test"},

			wantedExitCode: 217,
			wantedOut:      "",
			wantedErrOut:   "Error: test exec error\n",
		},
		"execute returns status code empty error": {
			exec: func(ctx context.Context, arguments []string) error {
				return ErrWithStatusCode(errors.New(""), 217)
			},

			args: []string{"test"},

			wantedExitCode: 217,
			wantedOut:      "",
			wantedErrOut:   "",
		},
		"initialize returns help requested error": {
			init: func() error {
				return ErrWithHelpRequested(errors.New("test init help error"))
			},

			args: []string{"test"},

			wantedExitCode: ExitCodeUsageError,
			wantedOut:      "",
			wantedErrOut:   "Error: test init help error\n\n" + singleHelpOut,
		},
		"initialize returns help requested empty error": {
			init: func() error {
				return ErrWithHelpRequested(errors.New(""))
			},

			args: []string{"test"},

			wantedExitCode: ExitCodeUsageError,
			wantedOut:      "",
			wantedErrOut:   singleHelpOut,
		},
		"execute returns help requested error": {
			exec: func(ctx context.Context, arguments []string) error {
				return ErrWithHelpRequested(errors.New("test exec help error"))
			},

			args: []string{"test"},

			wantedExitCode: ExitCodeUsageError,
			wantedOut:      "",
			wantedErrOut:   "Error: test exec help error\n\n" + singleHelpOut,
		},
		"execute returns help requested empty error": {
			exec: func(ctx context.Context, arguments []string) error {
				return ErrWithHelpRequested(errors.New(""))
			},

			args: []string{"test"},

			wantedExitCode: ExitCodeUsageError,
			wantedOut:      "",
			wantedErrOut:   singleHelpOut,
		},
		"execute returns help requested error wrapping status code error": {
			exec: func(ctx context.Context, arguments []string) error {
				return ErrWithHelpRequested(ErrWithStatusCode(errors.New("test exec help error"), 3))
			},

			args: []string{"test"},

			wantedExitCode: 3,
			wantedOut:      "",
			wantedErrOut:   "Error: test exec help error\n\n" + singleHelpOut,
		},
		"execute returns status code error wrapping help requested error": {
			exec: func(ctx context.Context, arguments []string) error {
				return ErrWithStatusCode(ErrWithHelpRequested(errors.New("test exec help error")), 3)
			},

			args: []string{"test"},

			wantedExitCode: 3,
			wantedOut:      "",
			wantedErrOut:   "Error: test exec help error\n\n" + singleHelpOut,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			var out, errOut bytes.Buffer

			app := NewSingleCommandApp(testAppInfo, testData.exec, testData.flags, &out, &errOut)

			if testData.init != nil {
				app.OnInit(testData.init)
			}

			exitCode := app.Run(context.TODO(), testData.args)

			if exitCode != testData.wantedExitCode {
				t.Errorf("app.Run gave %v, wanted %v", exitCode, testData.wantedExitCode)
			}

			if out.String() != testData.wantedOut {
				t.Errorf("app.Run gave out %q, wanted %q", out.String(), testData.wantedOut)
			}

			if errOut.String() != testData.wantedErrOut {
				t.Errorf("app.Run gave errOut %q, wanted %q", errOut.String(), testData.wantedErrOut)
			}
		})
	}
}

func TestMultiCommandApp_Run(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
	out := io.Discard

	app := NewMultiCommandApp(testAppInfo, flagSet, out, out)

	testCommandInfo := CommandInfo{Name: "testcommand"}

	var executorCapture struct {
		ctx       context.Context
		arguments []string
	}
	executor := func(ctx context.Context, arguments []string) error {
		executorCapture.ctx = ctx
		executorCapture.arguments = arguments

		return nil
	}
	commandFlagSet := flag.NewFlagSet(testCommandInfo.Name, flag.ExitOnError)

	_ = app.SetCommand(testCommandInfo, executor, commandFlagSet)

	ctxTestKey := struct{ k string }{k: "test-key-for-testing"}
	ctxTestVal := "test context val"
	ctx := context.WithValue(context.TODO(), ctxTestKey, ctxTestVal)
	args := []string{testCommandInfo.Name, "testarg1", "testarg2"}
	wantedExitCode := 0

	initRan := false
	initFn := func() error {
		initRan = true
		return nil
	}

	app.OnInit(initFn)

	exitCode := app.Run(ctx, args)

	if exitCode != wantedExitCode {
		t.Errorf("app.Run gave %v, wanted %v", exitCode, wantedExitCode)
	}

	if !initRan {
		t.Error("app.Run didn't run init function")
	}

	if executorCapture.ctx.Value(ctxTestKey) != ctxTestVal {
		t.Errorf("app.Run executor gave %q, wanted %q", executorCapture.ctx.Value(ctxTestKey), ctxTestVal)
	}

	if executorCapture.arguments[0] != args[1] && executorCapture.arguments[1] != args[2] {
		t.Errorf("app.Run executor gave %q, wanted %q", executorCapture.arguments, args)
	}
}

func TestMultiCommandApp_Run_EmptyArgsProvided(t *testing.T) {
	flagSet := flag.NewFlagSet(testAppInfo.Name, flag.ExitOnError)
	out := io.Discard

	app := NewMultiCommandApp(testAppInfo, flagSet, out, out)

	testCommandInfo := CommandInfo{Name: "testcommand"}

	var capturedArgs []string
	executor := func(ctx context.Context, arguments []string) error {
		capturedArgs = arguments
		return nil
	}

	commandFlagSet := flag.NewFlagSet(testCommandInfo.Name, flag.ExitOnError)

	_ = app.SetCommand(testCommandInfo, executor, commandFlagSet)

	originalOSArgs := os.Args[:]
	defer func() {
		// Put back the original args... to not mess with global state
		os.Args = originalOSArgs
	}()

	os.Args = []string{testAppInfo.Name, testCommandInfo.Name, "arg"}
	expectedArgs := os.Args[2:]

	if exitCode := app.Run(context.TODO(), nil); exitCode != 0 {
		t.Errorf("app.Run gave non-zero exit code %v", exitCode)
	}

	if capturedArgs[0] != expectedArgs[0] {
		t.Errorf("app.Run executor gave args %q, wanted %q", capturedArgs, expectedArgs)
	}
}

func TestMultiCommandApp_Run_AltPaths(t *testing.T) {
	testCommandInfo := CommandInfo{
		Name:    "testcommand",
		Summary: "A test command",
		Usage:   "args here...",
	}

	cmdHelpOut := fmt.Sprintf(`Usage: test testcommand args here...

A test command

Options:

  -help
    	Display the help message

test vTest (%s/%s)
`, runtime.GOOS, runtime.GOARCH)

	for testName, testData := range map[string]struct {
		flags        Flags
		exec         Executor
		commandFlags Flags
		init         func() error

		args []string

		wantedExitCode int
		wantedOut      string
		wantedErrOut   string
	}{
		"version requested": {
			args: []string{"--version"},

			wantedExitCode: 0,
			wantedOut:      fmt.Sprintf("test vTest (%s/%s)\n", runtime.GOOS, runtime.GOARCH),
			wantedErrOut:   "",
		},
		"help requested": {
			args: []string{"--help"},

			wantedExitCode: 0,
			wantedOut:      "",
			wantedErrOut: fmt.Sprintf(`Usage: test testing

A test

Commands:

	testcommand	A test command

Options:

  -help
    	Display the help message
  -version
    	Display the application version

test vTest (%s/%s)
`, runtime.GOOS, runtime.GOARCH),
		},
		"command help requested": {
			args: []string{testCommandInfo.Name, "--help"},

			wantedExitCode: 0,
			wantedOut:      "",
			wantedErrOut: fmt.Sprintf(`Usage: test testcommand args here...

A test command

Options:

  -help
    	Display the help message

test vTest (%s/%s)
`, runtime.GOOS, runtime.GOARCH),
		},
		"empty args": {
			args: []string{},

			wantedExitCode: 2,
			wantedOut:      "",
			wantedErrOut: fmt.Sprintf(`Usage: test testing

A test

Commands:

	testcommand	A test command

Options:

  -help
    	Display the help message
  -version
    	Display the application version

test vTest (%s/%s)
`, runtime.GOOS, runtime.GOARCH),
		},
		"args contain non-defined flags": {
			flags: flag.NewFlagSet("test", flag.ContinueOnError),
			args:  []string{"--non-existent-flag=val"},

			wantedExitCode: 2,
			wantedOut:      "",
			wantedErrOut: `Error: flag provided but not defined: -non-existent-flag

Usage: test testing

Run 'test --help' for usage.
`,
		},
		"initialize returns error": {
			init: func() error {
				return errors.New("test init error")
			},

			args: []string{testCommandInfo.Name},

			wantedExitCode: 1,
			wantedOut:      "",
			wantedErrOut:   "Error: test init error\n",
		},
		"initialize returns status code error": {
			init: func() error {
				return ErrWithStatusCode(errors.New("test init error"), 101)
			},

			args: []string{testCommandInfo.Name},

			wantedExitCode: 101,
			wantedOut:      "",
			wantedErrOut:   "Error: test init error\n",
		},
		"execute returns error": {
			exec: func(ctx context.Context, arguments []string) error {
				return errors.New("test exec error")
			},

			args: []string{testCommandInfo.Name},

			wantedExitCode: 1,
			wantedOut:      "",
			wantedErrOut:   "Error: test exec error\n",
		},
		"execute returns status code error": {
			exec: func(ctx context.Context, arguments []string) error {
				return ErrWithStatusCode(errors.New("test exec error"), 217)
			},

			args: []string{testCommandInfo.Name},

			wantedExitCode: 217,
			wantedOut:      "",
			wantedErrOut:   "Error: test exec error\n",
		},
		"unknown command": {
			args: []string{"thiscommanddoesnotexist"},

			wantedExitCode: 1,
			wantedOut:      "",
			wantedErrOut:   "Error: unknown command 'thiscommanddoesnotexist'\n\nUsage: test testing\n\nRun 'test --help' for usage.\n",
		},
		"unknown command is known flag": {
			flags: func() Flags {
				flags := flag.NewFlagSet("test", flag.ContinueOnError)

				flags.Bool("testflag", false, "some test flag")

				return flags
			}(),
			args: []string{"--testflag"},

			wantedExitCode: 1,
			wantedOut:      "",
			wantedErrOut:   "Error: unknown command '--testflag'\n\nUsage: test testing\n\nRun 'test --help' for usage.\n",
		},
		"initialize returns help requested error": {
			init: func() error {
				return ErrWithHelpRequested(errors.New("test init help error"))
			},

			args: []string{testCommandInfo.Name},

			wantedExitCode: ExitCodeUsageError,
			wantedOut:      "",
			wantedErrOut:   "Error: test init help error\n\n" + cmdHelpOut,
		},
		"initialize returns help requested empty error": {
			init: func() error {
				return ErrWithHelpRequested(errors.New(""))
			},

			args: []string{testCommandInfo.Name},

			wantedExitCode: ExitCodeUsageError,
			wantedOut:      "",
			wantedErrOut:   cmdHelpOut,
		},
		"execute returns help requested error": {
			exec: func(ctx context.Context, arguments []string) error {
				return ErrWithHelpRequested(errors.New("test exec help error"))
			},

			args: []string{testCommandInfo.Name},

			wantedExitCode: ExitCodeUsageError,
			wantedOut:      "",
			wantedErrOut:   "Error: test exec help error\n\n" + cmdHelpOut,
		},
		"execute returns help requested empty error": {
			exec: func(ctx context.Context, arguments []string) error {
				return ErrWithHelpRequested(errors.New(""))
			},

			args: []string{testCommandInfo.Name},

			wantedExitCode: ExitCodeUsageError,
			wantedOut:      "",
			wantedErrOut:   cmdHelpOut,
		},
		"execute returns help requested error wrapping status code error": {
			exec: func(ctx context.Context, arguments []string) error {
				return ErrWithHelpRequested(ErrWithStatusCode(errors.New("test exec help error"), 3))
			},

			args: []string{testCommandInfo.Name},

			wantedExitCode: 3,
			wantedOut:      "",
			wantedErrOut:   "Error: test exec help error\n\n" + cmdHelpOut,
		},
		"execute returns status code error wrapping help requested error": {
			exec: func(ctx context.Context, arguments []string) error {
				return ErrWithStatusCode(ErrWithHelpRequested(errors.New("test exec help error")), 3)
			},

			args: []string{testCommandInfo.Name},

			wantedExitCode: 3,
			wantedOut:      "",
			wantedErrOut:   "Error: test exec help error\n\n" + cmdHelpOut,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			var out, errOut bytes.Buffer

			app := NewMultiCommandApp(testAppInfo, testData.flags, &out, &errOut)

			_ = app.SetCommand(testCommandInfo, testData.exec, testData.commandFlags)

			if testData.init != nil {
				app.OnInit(testData.init)
			}

			exitCode := app.Run(context.TODO(), testData.args)

			if exitCode != testData.wantedExitCode {
				t.Errorf("app.Run gave %v, wanted %v", exitCode, testData.wantedExitCode)
			}

			if out.String() != testData.wantedOut {
				t.Errorf("app.Run gave out %q, wanted %q", out.String(), testData.wantedOut)
			}

			if errOut.String() != testData.wantedErrOut {
				t.Errorf("app.Run gave errOut %q, wanted %q", errOut.String(), testData.wantedErrOut)
			}
		})
	}
}
