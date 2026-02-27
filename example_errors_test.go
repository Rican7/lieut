// Copyright © 2026 Trevor N. Suarez (Rican7)

package lieut_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/Rican7/lieut"
)

func Example_errWithHelpRequested() {
	do := func(ctx context.Context, arguments []string) error {
		if len(arguments) == 0 {
			return lieut.ErrWithHelpRequested(errors.New("at least one argument is required"))
		}

		_, err := fmt.Println(arguments)

		return err
	}

	app := lieut.NewSingleCommandApp(
		lieut.AppInfo{Name: "example"},
		do,
		flag.CommandLine,
		os.Stdout,
		os.Stderr,
	)

	exitCode := app.Run(context.Background(), os.Args[1:])

	os.Exit(exitCode)
}

func Example_errWithStatusCode() {
	do := func(ctx context.Context, arguments []string) error {
		if len(arguments) == 0 {
			return lieut.ErrWithStatusCode(errors.New("at least one argument is required"), 2)
		}

		_, err := fmt.Println(arguments)

		return err
	}

	app := lieut.NewSingleCommandApp(
		lieut.AppInfo{Name: "example"},
		do,
		flag.CommandLine,
		os.Stdout,
		os.Stderr,
	)

	exitCode := app.Run(context.Background(), os.Args[1:])

	os.Exit(exitCode)
}

func Example_errWithHelpRequestedAndStatusCode() {
	do := func(ctx context.Context, arguments []string) error {
		if len(arguments) == 0 {
			return lieut.ErrWithStatusCode(
				lieut.ErrWithHelpRequested(errors.New("at least one argument is required")),
				2,
			)
		}

		_, err := fmt.Println(arguments)

		return err
	}

	app := lieut.NewSingleCommandApp(
		lieut.AppInfo{Name: "example"},
		do,
		flag.CommandLine,
		os.Stdout,
		os.Stderr,
	)

	exitCode := app.Run(context.Background(), os.Args[1:])

	os.Exit(exitCode)
}
