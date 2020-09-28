// Copyright 2018 The gg Authors
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

	"gg-scm.io/tool/internal/flag"
)

const diffSynopsis = "diff repository (or selected files)"

func diff(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg diff [--stat] [-c REV | -r REV1 [-r REV2]] [FILE [...]]", diffSynopsis)
	ignoreSpaceChange := f.Bool("b", false, "ignore changes in amount of whitespace")
	f.Alias("b", "ignore-space-change")
	ignoreBlankLines := f.Bool("B", false, "ignore changes whose lines are all blank")
	f.Alias("B", "ignore-blank-lines")
	change := f.String("c", "", "change made by `rev`ision")
	ncontext := f.Int("U", 3, "number of lines of context to show")
	var rev revFlag
	f.Var(&rev, "r", "`rev`ision")
	stat := f.Bool("stat", false, "output diffstat-style summary of changes")
	ignoreAllSpace := f.Bool("w", false, "ignore whitespace when comparing lines")
	f.Alias("w", "ignore-all-space")
	ignoreSpaceAtEOL := f.Bool("Z", false, "ignore changes in whitespace at EOL")
	f.Alias("Z", "ignore-space-at-eol")
	renames := f.String("M", "50%", "report new files with the set `percent`age of similarity to a removed file as renamed")
	copies := f.String("C", "50%", "report new files with the set `percent`age of similarity as copied")
	copiesUnmodified := f.Bool("copies-unmodified", true, "whether to check unmodified files when detecting copies (can be expensive)")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	var diffArgs []string
	diffArgs = append(diffArgs, "diff")
	if *stat {
		diffArgs = append(diffArgs, "--stat")
	} else {
		diffArgs = append(diffArgs, fmt.Sprintf("-U%d", *ncontext))
	}
	if *ignoreSpaceChange {
		diffArgs = append(diffArgs, "--ignore-space-change")
	}
	if *ignoreBlankLines {
		diffArgs = append(diffArgs, "--ignore-blank-lines")
	}
	if *ignoreAllSpace {
		diffArgs = append(diffArgs, "--ignore-all-space")
	}
	if *ignoreSpaceAtEOL {
		diffArgs = append(diffArgs, "--ignore-space-at-eol")
	}
	if *renames != "" {
		diffArgs = append(diffArgs, "--find-renames="+*renames)
	}
	if *copies != "" {
		diffArgs = append(diffArgs, "--find-copies="+*copies)
	}
	if *copiesUnmodified {
		diffArgs = append(diffArgs, "--find-copies-harder")
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
		if rev, err := cc.git.Head(ctx); err == nil {
			diffArgs = append(diffArgs, rev.Commit.String())
		} else {
			// HEAD not found; repository has not been initialized.
			// Compare to the null tree.

			// Run connects stdin to /dev/null.
			zeroHash, err := cc.git.NullTreeHash(ctx)
			if err != nil {
				return err
			}
			diffArgs = append(diffArgs, zeroHash.String())
		}
	}
	diffArgs = append(diffArgs, "--")
	diffArgs = append(diffArgs, f.Args()...)
	return cc.interactiveGit(ctx, diffArgs...)
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
