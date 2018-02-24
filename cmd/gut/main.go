// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"zombiezen.com/go/gut/internal/flag"
	"zombiezen.com/go/gut/internal/gittool"
)

func main() {
	pctx, err := osProcessContext()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gut:", err)
		os.Exit(1)
	}
	err = run(context.Background(), pctx, os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "gut:", err)
		if _, ok := err.(*usageError); ok {
			os.Exit(64)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, pctx *processContext, args []string) error {
	const synopsis = "gut [options] <command> [ARG [...]]"
	const description = "Git that comes from the Gut\n\n" +
		"basic commands:\n" +
		"  add           " + addSynopsis + "\n" +
		"  branch        " + branchSynopsis + "\n" +
		"  commit        " + commitSynopsis + "\n" +
		"  diff          " + diffSynopsis + "\n" +
		"  init          " + initSynopsis + "\n" +
		"  log           " + logSynopsis + "\n" +
		"  pull          " + pullSynopsis + "\n" +
		"  push          " + pushSynopsis + "\n" +
		"  status        " + statusSynopsis + "\n" +
		"  update        " + updateSynopsis

	globalFlags := flag.NewFlagSet(false, synopsis, description)
	gitPath := globalFlags.String("git", "", "`path` to git executable")
	showArgs := globalFlags.Bool("show-git", false, "log git invocations")
	if err := globalFlags.Parse(args); flag.IsHelp(err) {
		globalFlags.Help(pctx.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if globalFlags.NArg() == 0 {
		globalFlags.Help(pctx.stdout)
		return nil
	}
	if *gitPath == "" {
		var err error
		*gitPath, err = pctx.lookPath("git")
		if err != nil {
			return err
		}
	}
	opts := gittool.Options{
		Env:    pctx.env,
		Stdin:  pctx.stdin,
		Stdout: pctx.stdout,
		Stderr: pctx.stderr,
	}
	if *showArgs {
		opts.LogHook = func(_ context.Context, args []string) {
			var buf bytes.Buffer
			buf.WriteString("gut: exec: git")
			for _, a := range args {
				buf.WriteByte(' ')
				if strings.IndexByte(a, ' ') == -1 {
					buf.WriteString(a)
				} else {
					buf.WriteByte('"')
					buf.WriteString(a)
					buf.WriteByte('"')
				}
			}
			buf.WriteByte('\n')
			pctx.stderr.Write(buf.Bytes())
		}
	}
	git, err := gittool.New(*gitPath, pctx.dir, &opts)
	if err != nil {
		return err
	}
	cc := &cmdContext{
		git:    git,
		stdout: pctx.stdout,
		stderr: pctx.stderr,
	}
	return dispatch(ctx, cc, globalFlags, globalFlags.Arg(0), globalFlags.Args()[1:])
}

type cmdContext struct {
	git    *gittool.Tool
	stdout io.Writer
	stderr io.Writer
}

func dispatch(ctx context.Context, cc *cmdContext, globalFlags *flag.FlagSet, name string, args []string) error {
	switch name {
	case "add":
		return add(ctx, cc, args)
	case "branch":
		return branch(ctx, cc, args)
	case "commit", "ci":
		return commit(ctx, cc, args)
	case "diff":
		return diff(ctx, cc, args)
	case "init":
		return init_(ctx, cc, args)
	case "log", "history":
		return log(ctx, cc, args)
	case "status", "st", "check":
		return status(ctx, cc, args)
	case "pull":
		return pull(ctx, cc, args)
	case "push":
		return push(ctx, cc, args)
	case "update", "up", "checkout", "co":
		return update(ctx, cc, args)
	case "help":
		if len(args) == 0 {
			globalFlags.Help(cc.stdout)
			return nil
		}
		if len(args) > 1 || strings.HasPrefix(args[0], "-") {
			return usagef("help [command]")
		}
		return dispatch(ctx, cc, globalFlags, args[0], []string{"--help"})
	default:
		return usagef("unknown command %s", name)
	}
}

type processContext struct {
	dir string
	env []string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	lookPath func(string) (string, error)
}

func osProcessContext() (*processContext, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &processContext{
		dir:      dir,
		env:      os.Environ(),
		stdin:    os.Stdin,
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		lookPath: exec.LookPath,
	}, nil
}

type usageError string

func usagef(format string, args ...interface{}) error {
	e := usageError(fmt.Sprintf(format, args...))
	return &e
}

func (ue *usageError) Error() string {
	return "usage: " + string(*ue)
}
