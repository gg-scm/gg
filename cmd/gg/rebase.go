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

	"zombiezen.com/go/gg/internal/flag"
)

const rebaseSynopsis = "move revision (and descendants) to a different branch"

func rebase(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg rebase [--src REV | --base REV] [--dst REV] [options]", rebaseSynopsis)
	base := f.String("base", "", "rebase everything from branching point of specified `rev`ision")
	dst := f.String("dst", "@{upstream}", "rebase onto the specified `rev`ision")
	src := f.String("src", "", "rebase the specified `rev`ision and descendants")
	abort := f.Bool("abort", false, "abort an interrupted rebase")
	continue_ := f.Bool("continue", false, "continue an interrupted rebase")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() != 0 {
		return usagef("no arguments expected")
	}
	if *abort && *continue_ {
		return usagef("can't specify both --abort and --continue")
	}
	if (*abort || *continue_) && (*base != "" || *dst != "@{upstream}" || *src != "") {
		return usagef("can't specify other options with --abort or --continue")
	}
	if *abort {
		return cc.git.RunInteractive(ctx, "rebase", "--abort")
	}
	if *continue_ {
		return cc.git.RunInteractive(ctx, "rebase", "--continue")
	}
	var rebaseArgs []string
	rebaseArgs = append(rebaseArgs, "rebase", "--onto="+*dst)
	switch {
	case *base != "" && *src != "":
		return usagef("can't specify both -s and -b")
	case *base != "":
		rebaseArgs = append(rebaseArgs, "--fork-point", "--", *base)
	case *src != "":
		rebaseArgs = append(rebaseArgs, "--no-fork-point", "--", *src+"~")
	}
	return cc.git.RunInteractive(ctx, rebaseArgs...)
}

const histeditSynopsis = "interactively edit revision history"

func histedit(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg histedit [options] [ANCESTOR]", histeditSynopsis)
	abort := f.Bool("abort", false, "abort an edit already in progress")
	continue_ := f.Bool("continue", false, "continue an edit already in progress")
	editPlan := f.Bool("edit-plan", false, "edit remaining actions list")
	exec := f.MultiString("exec", "execute the shell `command` after each line creating a commit (can be specified multiple times)")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	switch {
	case !*abort && !*continue_ && !*editPlan:
		if f.NArg() > 1 {
			return usagef("no more than one ancestor should be given")
		}
		if ancestor := f.Arg(0); ancestor != "" {
			return cc.git.RunInteractive(ctx, "rebase", "-i", "--", ancestor)
		}
		rebaseArgs := []string{"rebase", "-i"}
		for _, cmd := range *exec {
			rebaseArgs = append(rebaseArgs, "--exec="+cmd)
		}
		return cc.git.RunInteractive(ctx, rebaseArgs...)
	case *abort && !*continue_ && !*editPlan:
		if f.NArg() != 0 {
			return usagef("can't pass arguments with --abort")
		}
		return cc.git.RunInteractive(ctx, "rebase", "--abort")
	case !*abort && *continue_ && !*editPlan:
		if f.NArg() != 0 {
			return usagef("can't pass arguments with --continue")
		}
		return cc.git.RunInteractive(ctx, "rebase", "--continue")
	case !*abort && !*continue_ && *editPlan:
		if f.NArg() != 0 {
			return usagef("can't pass arguments with --edit-todo")
		}
		return cc.git.RunInteractive(ctx, "rebase", "--edit-todo")
	default:
		return usagef("must specify at most one of --abort, --continue, or --edit-plan")
	}
}
