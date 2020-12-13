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

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
)

const backoutSynopsis = "reverse effect of an earlier commit"

func backout(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg backout [options] [-r] REV", backoutSynopsis+`

	Prepare a new commit with the effect of `+"`REV`"+` undone in the current
	working copy. If no conflicts were encountered, it will be committed
	immediately (unless `+"`-n`"+` is passed).`)
	edit := f.Bool("e", true, "invoke editor on commit message")
	f.Alias("e", "edit")
	noCommit := f.Bool("n", false, "do not commit")
	f.Alias("n", "no-commit")
	rev := f.String("r", "", "`rev`ision")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	var r *git.Rev
	switch {
	case f.NArg() == 0 && *rev != "":
		var err error
		r, err = cc.git.ParseRev(ctx, *rev)
		if err != nil {
			return err
		}
	case f.NArg() == 1 && *rev == "":
		var err error
		r, err = cc.git.ParseRev(ctx, f.Arg(0))
		if err != nil {
			return err
		}
	default:
		return usagef("must pass a single revision")
	}
	switch {
	case *noCommit:
		return cc.git.Run(ctx, "revert", "--no-commit", r.Commit.String())
	case *edit:
		// TODO(someday): Use our editor by running --no-commit and then
		// immediately running commit.
		return cc.interactiveGit(ctx, "revert", "--edit", r.Commit.String())
	default:
		return cc.git.Run(ctx, "revert", "--no-edit", r.Commit.String())
	}
}
