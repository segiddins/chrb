package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/segiddins/chrb"
	"github.com/urfave/cli"
	"golang.org/x/term"
)

type runResult struct {
	pattern string
	ruby    *chrb.Ruby
	stdout  string
	exit    string
	time    time.Duration
	err     error
}

func runRubies(rubies []string, command cli.Args) error {
	envs := []struct {
		env  []string
		ruby *chrb.Ruby
	}{}

	for _, ruby := range rubies {
		ruby, err := chrb.FindRuby(ruby)
		if err != nil {
			return err
		}
		env, err := ruby.Env()
		if err != nil {
			return err
		}
		envs = append(envs, struct {
			env  []string
			ruby *chrb.Ruby
		}{env, &ruby})
	}

	environ := os.Environ()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	go func() {
		select {
		case <-signals:
			fmt.Println("Interrupted")
			cancel()
		case <-ctx.Done():
		}
	}()

	results := make(chan runResult)

	for i, env := range envs {
		pattern := rubies[i]
		go func(env struct {
			env  []string
			ruby *chrb.Ruby
		}) {
			cmd := exec.CommandContext(ctx, env.ruby.ExecPath(), []string(command)...)
			cmd.Env = append(environ, env.env...)

			stdout := bytes.NewBuffer(nil)
			cmd.Stdout = stdout
			cmd.Stderr = stdout

			start := time.Now()
			err := cmd.Run()
			duration := time.Since(start)

			results <- runResult{pattern, env.ruby, stdout.String(), cmd.ProcessState.String(), duration, err}
		}(env)
	}

	resultsSlice := []runResult{}

	for range rubies {
		resultsSlice = append(resultsSlice, <-results)
	}

	errs := []error{}

	sort.Slice(resultsSlice, func(i, j int) bool {
		return resultsSlice[i].pattern < resultsSlice[j].pattern
	})

	width, _, _ := term.GetSize(int(os.Stdout.Fd()))
	width -= 2
	header := strings.Repeat("*", width)

	for i, result := range resultsSlice {
		if result.err != nil {
			errs = append(errs, multierror.Prefix(result.err, result.pattern))
		}
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(header)
		label := fmt.Sprintf(" %s (%s %s) -> %s in %s ", result.pattern, result.ruby.Engine, result.ruby.Version, result.exit, result.time)
		label = strings.Repeat("-", (width-len(label))/2) + label + strings.Repeat("-", (width-len(label))/2)
		fmt.Println(label)
		os.Stdout.WriteString(result.stdout)
		fmt.Println(label)
		fmt.Println(header)
	}

	if len(errs) > 0 {
		return multierror.Append(nil, errs...)
	}

	return nil
}

func main() {
	app := &cli.App{
		Name:  "chrb-matrix",
		Usage: "run ruby commands in a matrix of rubies",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "ruby",
				Usage: "the rubies to run the command on",
			},
		},
		Action: func(cCtx *cli.Context) error {
			rubies := cCtx.StringSlice("ruby")
			return runRubies(rubies, cCtx.Args())
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println("\n", err)
		os.Exit(1)
	}
}
