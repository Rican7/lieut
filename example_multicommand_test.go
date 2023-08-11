// Copyright © 2023 Trevor N. Suarez (Rican7)

package lieut_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Rican7/lieut"
)

var multiAppInfo = lieut.AppInfo{
	Name:    "now",
	Summary: "An example CLI app to report the date and time",
	Usage:   "<command>... [options]...",
	Version: "v1.0.4",
}

var (
	timeZone       = "UTC"
	includeSeconds = false
	includeYear    = false
)

var location *time.Location

func Example_multiCommand() {
	globalFlags := flag.NewFlagSet(multiAppInfo.Name, flag.ExitOnError)
	globalFlags.StringVar(&timeZone, "timezone", timeZone, "the timezone to report in")

	app := lieut.NewMultiCommandApp(
		multiAppInfo,
		globalFlags,
		os.Stdout,
		os.Stderr,
	)

	timeFlags := flag.NewFlagSet("time", flag.ExitOnError)
	timeFlags.BoolVar(&includeSeconds, "seconds", includeSeconds, "to include seconds")

	dateFlags := flag.NewFlagSet("date", flag.ExitOnError)
	dateFlags.BoolVar(&includeYear, "year", includeYear, "to include year")

	app.SetCommand(lieut.CommandInfo{Name: "time", Summary: "Show the time", Usage: "[options]"}, printTime, timeFlags)
	app.SetCommand(lieut.CommandInfo{Name: "date", Summary: "Show the date", Usage: "[options]"}, printDate, dateFlags)

	app.OnInit(validateGlobals)

	exitCode := app.Run(context.Background(), os.Args[1:])

	os.Exit(exitCode)
}

func validateGlobals() error {
	var err error

	location, err = time.LoadLocation(timeZone)

	return err
}

func printTime(ctx context.Context, arguments []string) error {
	format := "15:04"

	if includeSeconds {
		format += ":05"
	}

	_, err := fmt.Println(time.Now().Format(format))

	return err
}

func printDate(ctx context.Context, arguments []string) error {
	format := "01-02"

	if includeYear {
		format += "-2006"
	}

	_, err := fmt.Println(time.Now().Format(format))

	return err
}
