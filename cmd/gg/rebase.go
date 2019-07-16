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
	"strings"

	"gg-scm.io/pkg/internal/escape"
	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/git"
	"gg-scm.io/pkg/internal/sigterm"
)

const rebaseSynopsis = "move revision (and descendants) to a different branch"

func rebase(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg rebase [--src REV | --base REV] [--dst REV] [options]", rebaseSynopsis+`

	Rebasing will replay a set of changes on top of the destination
	revision and set the current branch to the final revision.

	If neither `+"`--src`"+` or `+"`--base`"+` is specified, it acts as if
	`+"`--base=@{upstream}`"+` was specified.`)
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
		c := cc.git.Command(ctx, "rebase", "--abort")
		c.Stdin = cc.stdin
		c.Stdout = cc.stdout
		c.Stderr = cc.stderr
		return sigterm.Run(ctx, c)
	}
	if *continue_ {
		return continueRebase(ctx, cc)
	}
	switch {
	case *base != "" && *src != "":
		return usagef("can't specify both -s and -b")
	case *base != "":
		c := cc.git.Command(ctx, "rebase", "--onto="+*dst, "--no-fork-point", "--", *base)
		c.Stdin = cc.stdin
		c.Stdout = cc.stdout
		c.Stderr = cc.stderr
		return sigterm.Run(ctx, c)
	case *src != "":
		if strings.HasPrefix(*src, "-") {
			return fmt.Errorf("revision cannot start with '-'")
		}
		ancestor, err := cc.git.IsAncestor(ctx, *src, git.Head.String())
		if err != nil {
			return err
		}
		if ancestor {
			// Simple case: this is an ancestor revision.
			c := cc.git.Command(ctx, "rebase", "--onto="+*dst, "--no-fork-point", "--", *src+"~")
			c.Stdin = cc.stdin
			c.Stdout = cc.stdout
			c.Stderr = cc.stderr
			return sigterm.Run(ctx, c)
		}

		// More complicated: this is on an unrelated branch.
		//
		// Non-interactive git rebase does not permit this, so we have to
		// kick off an interactive rebase with the plan we want.
		descend, err := findDescendants(ctx, cc.git, *src)
		if err != nil {
			return err
		}
		if len(descend) == 0 {
			return fmt.Errorf("%s is not part of any branch", *src)
		}
		if len(descend) > 1 {
			return fmt.Errorf("%s is in multiple branches", *src)
		}
		editorCmd := fmt.Sprintf(
			"%s log --reverse --first-parent --pretty='tformat:pick %%H' %s~..%s >",
			escape.Shell(cc.git.Path()), escape.Shell(*src), escape.Shell(descend[0].String()))
		c := cc.git.Command(ctx,
			"-c", "sequence.editor="+editorCmd,
			"rebase",
			"-i",
			"--onto="+*dst,
			"--no-fork-point",
			git.Head.String())
		c.Stdin = cc.stdin
		c.Stdout = cc.stdout
		c.Stderr = cc.stderr
		return sigterm.Run(ctx, c)
	default:
		c := cc.git.Command(ctx, "rebase", "--onto="+*dst, "--no-fork-point")
		c.Stdin = cc.stdin
		c.Stdout = cc.stdout
		c.Stderr = cc.stderr
		return sigterm.Run(ctx, c)
	}
}

const histeditSynopsis = "interactively edit revision history"

func histedit(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg histedit [options] [UPSTREAM]", histeditSynopsis+`

	This command lets you interactively edit a linear series of commits.
	When starting `+"`histedit`"+`, it will open your editor to plan the series
	of changes you want to make. You can reorder commits, or use the
	actions listed in the plan comments.

	Unlike `+"`git rebase -i`"+`, continuing a `+"`histedit`"+` will automatically
	amend the current commit if any changes are made. In most cases,
	you do not need to run `+"`commit --amend`"+` yourself.`)
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
		upstream := f.Arg(0)
		if strings.HasPrefix(upstream, "-") {
			return errors.New("upstream ref cannot start with a dash")
		}
		if upstream == "" {
			upstream = "@{upstream}"
		}
		mergeBase, err := cc.git.MergeBase(ctx, upstream, git.Head.String())
		if err != nil {
			return err
		}
		rebaseArgs := []string{"rebase", "-i", "--onto=" + mergeBase.String(), "--no-fork-point"}
		for _, cmd := range *exec {
			rebaseArgs = append(rebaseArgs, "--exec="+cmd)
		}
		rebaseArgs = append(rebaseArgs, "--", mergeBase.String())
		c := cc.git.Command(ctx, rebaseArgs...)
		c.Stdin = cc.stdin
		c.Stdout = cc.stdout
		c.Stderr = cc.stderr
		return sigterm.Run(ctx, c)
	case *abort && !*continue_ && !*editPlan:
		if f.NArg() != 0 {
			return usagef("can't pass arguments with --abort")
		}
		c := cc.git.Command(ctx, "rebase", "--abort")
		c.Stdin = cc.stdin
		c.Stdout = cc.stdout
		c.Stderr = cc.stderr
		return sigterm.Run(ctx, c)
	case !*abort && *continue_ && !*editPlan:
		if f.NArg() != 0 {
			return usagef("can't pass arguments with --continue")
		}
		return continueRebase(ctx, cc)
	case !*abort && !*continue_ && *editPlan:
		if f.NArg() != 0 {
			return usagef("can't pass arguments with --edit-todo")
		}
		c := cc.git.Command(ctx, "rebase", "--edit-todo")
		c.Stdin = cc.stdin
		c.Stdout = cc.stdout
		c.Stderr = cc.stderr
		return sigterm.Run(ctx, c)
	default:
		return usagef("must specify at most one of --abort, --continue, or --edit-plan")
	}
}

// continueRebase adds any modified files to the index and then runs
// `git rebase --continue`.
func continueRebase(ctx context.Context, cc *cmdContext) error {
	status, err := cc.git.Status(ctx, git.StatusOptions{})
	if err != nil {
		return err
	}
	hasChanges, err := verifyNoMissingOrUnmerged(status)
	if err != nil {
		return err
	}
	if hasChanges {
		if err := cc.git.StageTracked(ctx); err != nil {
			return err
		}
	}
	c := cc.git.Command(ctx, "rebase", "--continue")
	c.Stdin = cc.stdin
	c.Stdout = cc.stdout
	c.Stderr = cc.stderr
	return sigterm.Run(ctx, c)
}

// findDescendants returns the set of distinct heads under refs/heads/
// that contain the given commit object.
func findDescendants(ctx context.Context, git *git.Git, object string) ([]git.Ref, error) {
	refs, err := branchesContaining(ctx, git, object)
	if err != nil {
		return nil, fmt.Errorf("find descendants of %s: %v", object, err)
	}
	n := 0
	for i := range refs {
		others, err := branchesContaining(ctx, git, refs[i].String())
		if err != nil {
			return nil, fmt.Errorf("find descendants of %s: %v", object, err)
		}
		if len(others) == 0 {
			return nil, fmt.Errorf("find descendants of %s: inconsistent git output for %s", object, refs[i])
		}
		if len(others) > 1 {
			continue
		}
		if others[0] != refs[i] {
			return nil, fmt.Errorf("find descendants of %s: inconsistent git output for %s", object, refs[i])
		}
		refs[n] = refs[i]
		n++
	}
	return refs[:n], nil
}

// branchesContaining returns the set of refs under refs/heads/ that
// contain the given commit object. The order is undefined.
func branchesContaining(ctx context.Context, g *git.Git, object string) ([]git.Ref, error) {
	// TODO(soon): Turn this into an API.
	out, err := g.Output(ctx, "for-each-ref", "--contains="+object, "--format=%(refname)", "--", "refs/heads/*")
	if err != nil {
		return nil, fmt.Errorf("list branches: %v", err)
	}
	var refs []git.Ref
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		refs = append(refs, git.Ref(line))
	}
	return refs, nil
}
