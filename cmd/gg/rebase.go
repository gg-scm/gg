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
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gitobj"
	"zombiezen.com/go/gg/internal/gittool"
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
		return cc.git.RunInteractive(ctx, "rebase", "--abort")
	}
	if *continue_ {
		return continueRebase(ctx, cc.git)
	}
	switch {
	case *base != "" && *src != "":
		return usagef("can't specify both -s and -b")
	case *base != "":
		return cc.git.RunInteractive(ctx, "rebase", "--onto="+*dst, "--no-fork-point", "--", *base)
	case *src != "":
		if strings.HasPrefix(*src, "-") {
			return fmt.Errorf("revision cannot start with '-'")
		}
		ancestor, err := cc.git.Query(ctx, "merge-base", "--is-ancestor", *src, gitobj.Head.String())
		if err != nil {
			return err
		}
		if ancestor {
			// Simple case: this is an ancestor revision.
			return cc.git.RunInteractive(ctx, "rebase", "--onto="+*dst, "--no-fork-point", "--", *src+"~")
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
			shellEscape(cc.git.Path()), shellEscape(*src), shellEscape(descend[0].String()))
		return cc.git.RunInteractive(ctx,
			"-c", "sequence.editor="+editorCmd,
			"rebase",
			"-i",
			"--onto="+*dst,
			"--no-fork-point",
			gitobj.Head.String())
	default:
		return cc.git.RunInteractive(ctx, "rebase", "--onto="+*dst, "--no-fork-point")
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
		mergeBaseBytes, err := cc.git.RunOneLiner(ctx, '\n', "merge-base", upstream, gitobj.Head.String())
		if err != nil {
			return err
		}
		mergeBase, err := gitobj.ParseHash(string(mergeBaseBytes))
		if err != nil {
			return fmt.Errorf("parse merge base: %v", err)
		}
		rebaseArgs := []string{"rebase", "-i", "--onto=" + mergeBase.String(), "--no-fork-point"}
		for _, cmd := range *exec {
			rebaseArgs = append(rebaseArgs, "--exec="+cmd)
		}
		rebaseArgs = append(rebaseArgs, "--", mergeBase.String())
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
		return continueRebase(ctx, cc.git)
	case !*abort && !*continue_ && *editPlan:
		if f.NArg() != 0 {
			return usagef("can't pass arguments with --edit-todo")
		}
		return cc.git.RunInteractive(ctx, "rebase", "--edit-todo")
	default:
		return usagef("must specify at most one of --abort, --continue, or --edit-plan")
	}
}

// continueRebase adds any modified files to the index and then runs
// `git rebase --continue`.
func continueRebase(ctx context.Context, git *gittool.Tool) error {
	addArgs := []string{"add", "--"}
	fileStart := len(addArgs)
	var err error
	addArgs, err = inferCommitFiles(ctx, git, addArgs)
	if err != nil {
		return err
	}
	if len(addArgs) > fileStart {
		if err := git.RunInteractive(ctx, addArgs...); err != nil {
			return err
		}
	}
	return git.RunInteractive(ctx, "rebase", "--continue")
}

// findDescendants returns the set of distinct heads under refs/heads/
// that contain the given commit object.
func findDescendants(ctx context.Context, git *gittool.Tool, object string) ([]gitobj.Ref, error) {
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
func branchesContaining(ctx context.Context, git *gittool.Tool, object string) ([]gitobj.Ref, error) {
	p, err := git.Start(ctx, "for-each-ref", "--contains="+object, "--format=%(refname)", "--", "refs/heads/*")
	if err != nil {
		return nil, fmt.Errorf("list branches: %v", err)
	}
	defer p.Wait()
	s := bufio.NewScanner(p)
	var refs []gitobj.Ref
	for s.Scan() {
		refs = append(refs, gitobj.Ref(s.Text()))
	}
	if err := p.Wait(); err != nil {
		return nil, fmt.Errorf("list branches: %v", err)
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("list branches: %v", err)
	}
	return refs, nil
}

// shellEscape quotes s such that it can be used as a literal argument
// to a shell command.
func shellEscape(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for i := 0; i < len(s); i++ {
		if !isShellSafe(s[i]) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	sb := new(strings.Builder)
	sb.Grow(len(s) + 2)
	sb.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			sb.WriteString(`'\''`)
		} else {
			sb.WriteByte(s[i])
		}
	}
	sb.WriteByte('\'')
	return sb.String()
}

func isShellSafe(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-' || b == '_' || b == '/' || b == '.'
}
