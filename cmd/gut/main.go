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
	"context"
	"fmt"
	"os"
	"strings"

	"zombiezen.com/go/gut/internal/flag"
	"zombiezen.com/go/gut/internal/gittool"
)

var globalFlags flag.FlagSet

func main() {
	globalFlags.Init(false, "gut [options] <command> [ARG [...]]", `Git that comes from the Gut

basic commands:
  add           add the specified files on the next commit
  status        show changed files in the working directory`)
	gitPath := globalFlags.String("git", "", "`path` to git executable")
	if err := globalFlags.Parse(os.Args[1:]); flag.IsHelp(err) {
		globalFlags.Help(os.Stdout)
		return
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "gut: usage:", err)
		os.Exit(exitUsage)
	}
	if globalFlags.NArg() == 0 {
		globalFlags.Help(os.Stdout)
		return
	}
	var git *gittool.Tool
	var err error
	if *gitPath == "" {
		git, err = gittool.Find()
	} else {
		git, err = gittool.New(*gitPath)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gut:", err)
		os.Exit(exitFail)
	}
	sub := subcmds[globalFlags.Arg(0)]
	if sub == nil {
		fmt.Fprintln(os.Stderr, "gut: usage: unknown command", globalFlags.Arg(0))
		os.Exit(exitUsage)
	}
	ctx := context.Background()
	if err := sub(ctx, git, globalFlags.Args()[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gut:", err)
		os.Exit(errorExitCode(err))
	}
}

var subcmds map[string]func(context.Context, *gittool.Tool, []string) error

func init() {
	// Placed in init to break initialization loop.
	subcmds = map[string]func(context.Context, *gittool.Tool, []string) error{
		"add":    add,
		"check":  status,
		"help":   help,
		"st":     status,
		"status": status,
	}
}

func help(ctx context.Context, git *gittool.Tool, args []string) error {
	if len(args) == 0 {
		globalFlags.Help(os.Stdout)
		return nil
	}
	if len(args) > 1 || strings.HasPrefix(args[0], "-") {
		return usagef("help [command]")
	}
	sub := subcmds[args[0]]
	if sub == nil {
		return fmt.Errorf("no help found for %s", args[0])
	}
	return sub(ctx, git, []string{"--help"})
}

func add(ctx context.Context, git *gittool.Tool, args []string) error {
	f := flag.NewFlagSet(true, "gut add FILE [...]", "add the specified files on the next commit")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(os.Stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 {
		return usagef("must pass one or more files to add")
	}
	return git.Run(ctx, append([]string{"add", "-N", "--"}, f.Args()...)...)
}

type usageError string

func usagef(format string, args ...interface{}) error {
	e := usageError(fmt.Sprintf(format, args...))
	return &e
}

func (ue *usageError) Error() string {
	return "usage: " + string(*ue)
}

const (
	exitFail  = 1
	exitUsage = 64
)

func errorExitCode(e error) int {
	if _, ok := e.(*usageError); ok {
		return exitUsage
	}
	return exitFail
}
