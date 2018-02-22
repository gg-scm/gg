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
	"os"

	"zombiezen.com/go/gut/internal/flag"
	"zombiezen.com/go/gut/internal/gittool"
)

const updateSynopsis = "update working directory (or switch revisions)"

func update(ctx context.Context, git *gittool.Tool, args []string) error {
	f := flag.NewFlagSet(true, "gut update [-m] [[-r] REV]", updateSynopsis+"\n\n"+
		"aliases: up, checkout, co")
	merge := f.Bool("m", false, "merge uncommitted changes")
	f.Alias("m", "merge")
	rev := f.String("r", "", "`rev`ision")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(os.Stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	var r *gittool.Rev
	switch {
	case f.NArg() == 0 && *rev == "":
		// TODO(someday): how to apply --merge?
		return git.Run(ctx, "merge", "--quiet", "--ff-only")
	case f.NArg() == 0 && *rev != "":
		var err error
		r, err = gittool.ParseRev(ctx, git, *rev)
		if err != nil {
			return err
		}
	case f.NArg() == 1 && *rev == "":
		var err error
		r, err = gittool.ParseRev(ctx, git, f.Arg(0))
		if err != nil {
			return err
		}
	default:
		return usagef("can pass only one revision")
	}
	var coArgs []string
	coArgs = append(coArgs, "checkout", "--quiet")
	if *merge {
		coArgs = append(coArgs, "--merge")
	}
	if b := r.Branch(); b != "" {
		coArgs = append(coArgs, b)
	} else {
		coArgs = append(coArgs, "--detach", r.CommitHex())
	}
	coArgs = append(coArgs, "--")
	return git.Run(ctx, coArgs...)
}
