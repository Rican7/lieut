// Copyright © 2023 Trevor N. Suarez (Rican7)

package lieut_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Rican7/lieut"
)

var singleAppInfo = lieut.AppInfo{
	Name:    "sayhello",
	Summary: "An example CLI app to say hello to the given names",
	Usage:   "[option]... [names]...",
	Version: "v0.1-alpha",
}

func Example_singleCommand() {
	flagSet := flag.NewFlagSet(singleAppInfo.Name, flag.ExitOnError)

	app := lieut.NewSingleCommandApp(
		singleAppInfo,
		sayHello,
		flagSet,
		os.Stdout,
		os.Stderr,
	)

	exitCode := app.Run(context.Background(), os.Args[1:])

	os.Exit(exitCode)
}

func sayHello(ctx context.Context, arguments []string) error {
	names := strings.Join(arguments, ", ")
	hello := fmt.Sprintf("Hello %s!", names)

	_, err := fmt.Println(hello)

	return err
}
