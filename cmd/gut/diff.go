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
	"os"

	"zombiezen.com/go/gut/internal/flag"
	"zombiezen.com/go/gut/internal/gittool"
)

const diffSynopsis = "diff repository (or selected files)"

func diff(ctx context.Context, git *gittool.Tool, args []string) error {
	f := flag.NewFlagSet(true, "gut diff [--stat] [-c REV | -r REV1 [-r REV2]] [FILE [...]]", diffSynopsis)
	change := f.String("c", "", "change made by `rev`ision")
	var rev revFlag
	f.Var(&rev, "r", "`rev`ision")
	stat := f.Bool("stat", false, "output diffstat-style summary of changes")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(os.Stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	var diffArgs []string
	diffArgs = append(diffArgs, "diff")
	if *stat {
		diffArgs = append(diffArgs, "--stat")
	}
	switch {
	case rev.r1 != "" && *change == "":
		diffArgs = append(diffArgs, rev.r1)
		if rev.r2 != "" {
			diffArgs = append(diffArgs, rev.r2)
		}
	case rev.r1 == "" && *change != "":
		diffArgs = append(diffArgs, *change+"^", *change)
	case rev.r1 != "" && *change != "":
		return usagef("can't pass both -r and -c")
	default:
		diffArgs = append(diffArgs, "HEAD")
	}
	diffArgs = append(diffArgs, "--")
	diffArgs = append(diffArgs, f.Args()...)
	return git.RunInteractive(ctx, diffArgs...)
}

type revFlag struct {
	r1, r2 string
}

func (r *revFlag) String() string {
	if r.r2 == "" {
		return r.r1
	}
	return r.r1 + " " + r.r2
}

func (r *revFlag) Set(s string) error {
	if s == "" {
		return errors.New("blank revision")
	}
	if r.r1 == "" {
		r.r1 = s
		return nil
	}
	if r.r2 == "" {
		r.r2 = s
		return nil
	}
	return errors.New("can only pass a revision flag at most twice")
}

func (r *revFlag) Get() interface{} {
	return *r
}

func (r *revFlag) IsBoolFlag() bool {
	return false
}
