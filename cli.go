package chrb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

var configKey = &struct{}{}

func GetConfig(ctx context.Context) *Config {
	return ctx.Value(configKey).(*Config)
}

func App(config *Config) *cli.Command {
	return &cli.Command{
		Name:           "chrb",
		Usage:          "run ruby commands",
		DefaultCommand: "list-or-use",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "default-ruby-version",
				Usage: "default ruby to use when no ruby is specified",
				Sources: cli.NewValueSourceChain(
					cli.EnvVar("DEFAULT_RUBY_VERSION"),
				),
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			ctx = context.WithValue(ctx, configKey, config)
			return ctx, nil
		},
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list all installed rubies",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "format",
						Value: "text",
						Usage: "text|json",
					},
				},
				Action: listRubies,
			},
			{
				Name:      "list-or-use",
				Usage:     "list or use a ruby",
				ArgsUsage: "[ruby]",
				Hidden:    true,
				Action: func(ctx context.Context, cmd *cli.Command) error {
					fmt.Fprintf(cmd.Writer, "%#v\n", cmd.Root().Args())
					if cmd.Root().NArg() == 0 {
						return listRubies(ctx, cmd)
					}
					return useRuby(ctx, cmd.Root())
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "format",
						Value: "text",
						Usage: "text|json",
					},
				},
			},
			{
				Name:      "use",
				Usage:     "prints the shell commands to eval to use the ruby",
				ArgsUsage: "<ruby>",
				Action:    useRuby,
			},
			{
				Name:      "exec",
				Usage:     "execute a command with a ruby",
				ArgsUsage: "<ruby> <command>",
				Action:    execRuby,
			},
			{
				Name:  "matrix",
				Usage: "run a command in a matrix of rubies",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name: "command",
						Min:  1,
						Max:  1,
					},
					&cli.StringArg{
						Name: "arguments",
						Min:  0,
						Max:  -1,
					},
				},

				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:     "ruby",
						Usage:    "the rubies to run the command on",
						Required: true,
					},
				},
				Action: execMatrix,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return cli.ShowAppHelp(cmd)
		},
	}
}

func listRubies(ctx context.Context, cmd *cli.Command) error {
	config := GetConfig(ctx)

	rubies, err := ListRubies(config)
	if err != nil {
		return err
	}

	activeRoot := config.Env.Getenv("RUBY_ROOT")

	switch format := cmd.String("format"); format {
	case "json":
		return json.NewEncoder(cmd.Writer).Encode(rubies)
	case "text":
		for _, ruby := range rubies {
			activeString := " "
			if activeRoot == string(ruby.RubyDir) {
				activeString = "*"
			}

			fmt.Fprintf(cmd.Writer, " %s %s\n", activeString, filepath.Base(string(ruby.RubyDir)))
		}
	default:
		return fmt.Errorf("invalid format: %q", format)
	}

	return nil
}

func useRuby(ctx context.Context, cmd *cli.Command) error {
	config := GetConfig(ctx)

	if cmd.NArg() != 1 {
		return fmt.Errorf("usage: chrb use <ruby>")
	}
	pattern := cmd.Args().First()

	if found, err := FindRubyVersion(config, pattern); err == nil {
		pattern = found
	}

	env := config.Env.Clone()
	env.ResetRubyEnv(config.Uid)

	ruby, err := FindRuby(pattern, config)
	if err != nil {
		return err
	}

	env, err = ruby.Env(config)
	if err != nil {
		return err
	}

	for _, e := range env.Diff(config.Env.ToEnvList()) {
		if e.Value != nil {
			fmt.Fprintf(cmd.Writer, "export %s=%s\n", e.Key, strconv.Quote(*e.Value))
		} else {
			fmt.Fprintf(cmd.Writer, "unset %s\n", e.Key)
		}
	}

	return nil
}

func execRuby(ctx context.Context, cmd *cli.Command) error {
	config := GetConfig(ctx)

	if cmd.NArg() < 2 {
		return fmt.Errorf("usage: chrb exec <ruby> <command>")
	}
	pattern := cmd.Args().First()
	command := cmd.Args().Tail()

	env := config.Env.Clone()
	env.ResetRubyEnv(config.Uid)

	ruby, err := FindRuby(pattern, config)
	if err != nil {
		return err
	}

	env, err = ruby.Env(config)
	if err != nil {
		return err
	}

	command = append([]string{"chruby exec"}, command...)
	return syscall.Exec("/usr/bin/env", command, env.ToEnvList())
}

type runResult struct {
	pattern string
	ruby    *Ruby
	stdout  string
	exit    string
	time    time.Duration
	err     error
}

func (r *runResult) String() string {
	if r.time > 0 {
		return fmt.Sprintf("%s (%s %s) -> %s in %s", r.pattern, r.ruby.Engine, r.ruby.Version, r.exit, r.time.Round(time.Millisecond))
	}
	return fmt.Sprintf("%s (%s %s)", r.pattern, r.ruby.Engine, r.ruby.Version)
}

func execMatrix(ctx context.Context, cmd *cli.Command) error {
	rubies := cmd.StringSlice("ruby")

	config := GetConfig(ctx)
	env := config.Env.Clone()
	env.ResetRubyEnv(config.Uid)

	envs := []struct {
		env  []string
		ruby *Ruby
	}{}

	for _, ruby := range rubies {
		ruby, err := FindRuby(ruby, config)
		if err != nil {
			return err
		}
		env, err := ruby.Env(config)
		if err != nil {
			return err
		}
		envs = append(envs, struct {
			env  []string
			ruby *Ruby
		}{env.ToEnvList(), &ruby})
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	go func() {
		select {
		case <-signals:
			fmt.Fprintln(cmd.Writer, "Interrupted")
			cancel()
		case <-ctx.Done():
		}
	}()

	pw := progress.NewWriter()
	pw.SetAutoStop(true)
	pw.SetOutputWriter(cmd.Writer)
	pw.SetNumTrackersExpected(len(rubies))
	style := progress.StyleDefault
	style.Visibility.TrackerOverall = false
	style.Visibility.ETA = false
	style.Visibility.ETAOverall = false
	style.Visibility.Percentage = false
	style.Visibility.Value = false
	style.Options.TimeDonePrecision = time.Millisecond
	pw.SetStyle(style)
	go func() {
		pw.Render()
	}()

	results := make(chan runResult)

	arg := *cmd.Arguments[1].(*cli.StringArg).Values
	if len(arg) > 0 && arg[0] == "--" {
		arg = arg[1:]
	}
	arg0 := *cmd.Arguments[0].(*cli.StringArg).Values
	arg = append(arg0, arg...)

	for i, env := range envs {
		pattern := rubies[i]
		tracker := &progress.Tracker{
			Message: pattern,
			Total:   0,
		}
		pw.AppendTracker(tracker)
		go func(env struct {
			env  []string
			ruby *Ruby
		}) {
			result := &runResult{pattern: pattern, ruby: env.ruby}
			tracker.UpdateMessage(result.String())
			defer func() {
				tracker.UpdateMessage(result.String())
				if result.err != nil {
					tracker.MarkAsErrored()
				} else {
					tracker.MarkAsDone()
				}
			}()
			cmd := exec.CommandContext(ctx, "env", arg...)
			cmd.Env = env.env
			stdout := bytes.NewBuffer(nil)
			cmd.Stdout = stdout
			cmd.Stderr = stdout

			start := time.Now()
			err := cmd.Run()
			duration := time.Since(start)

			result.stdout = stdout.String()
			result.exit = cmd.ProcessState.String()
			result.time = duration
			result.err = err
			results <- *result
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

	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
	}
	width -= 2
	header := strings.Repeat("*", width)

	for i, result := range resultsSlice {
		if result.err != nil {
			errs = append(errs, multierror.Prefix(result.err, result.pattern))
		}
		if i > 0 {
			fmt.Fprintln(cmd.Writer)
		}
		fmt.Fprintln(cmd.Writer, header)
		label := result.String()
		label = strings.Repeat("-", (width-len(label))/2) + label + strings.Repeat("-", (width-len(label))/2)
		fmt.Fprintln(cmd.Writer, label)
		cmd.Writer.Write([]byte(result.stdout))
		fmt.Fprintln(cmd.Writer, label)
		fmt.Fprintln(cmd.Writer, header)
	}

	if len(errs) > 0 {
		return multierror.Append(nil, errs...)
	}

	return nil
}
