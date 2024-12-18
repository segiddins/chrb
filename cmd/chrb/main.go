package main

import (
	"context"
	"fmt"
	"os"

	"github.com/segiddins/chrb"
	"github.com/spf13/afero"
)

func main() {
	config := chrb.Config{
		Env:           chrb.ParseEnv(os.Environ()),
		Options:       chrb.DefaultOptions.Clone(),
		Uid:           os.Getuid(),
		Fs:            afero.NewOsFs(),
		RubyEnvFinder: chrb.ExecFindEnv,
	}
	app := chrb.App(&config)

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
