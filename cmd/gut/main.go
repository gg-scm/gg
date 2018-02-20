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
	"errors"
	"fmt"
	"os"

	"zombiezen.com/go/gut/internal/flag"
	"zombiezen.com/go/gut/internal/gittool"
)

func main() {
	f := flag.NewFlagSet(false)
	gitPath := f.String("git", "", "path to git executable")
	if err := f.Parse(os.Args[1:]); flag.IsHelpUndefined(err) {
		usage()
		return
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "gut: usage:", err)
		os.Exit(64)
	}
	if f.NArg() == 0 {
		usage()
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
		os.Exit(1)
	}
	ctx := context.Background()
	switch f.Arg(0) {
	case "add":
		err = add(ctx, git, f.Args()[1:])
	default:
		fmt.Fprintln(os.Stderr, "gut: usage: unknown command", f.Arg(0))
		os.Exit(64)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gut:", err)
		os.Exit(1)
	}
}

func add(ctx context.Context, git *gittool.Tool, args []string) error {
	f := new(flag.FlagSet)
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() == 0 {
		// TODO(soon): make into usage error
		return errors.New("must pass one or more files to add")
	}
	return git.Run(ctx, append([]string{"add", "-N", "--"}, f.Args()...)...)
}

func usage() {
	fmt.Println("Git that comes from the Gut")
	fmt.Println()
	fmt.Println("basic commands:")
	fmt.Println()
	fmt.Println("  add           add the specified files on the next commit")
}
