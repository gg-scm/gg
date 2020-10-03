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
	"sort"
	"strings"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
	"gg-scm.io/tool/internal/terminal"
)

const branchSynopsis = "list or manage branches"

func branch(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg branch [-d] [-f] [-r REV] [NAME [...]]", branchSynopsis+`

	Branches are references to commits to help track lines of
	development. Branches are unversioned and can be moved, renamed, and
	deleted.
	
	Creating or updating to a branch causes it to be marked as active.
	When a commit is made, the active branch will advance to the new
	commit. A plain `+"`gg update`"+` will also advance an active branch, if
	possible. If the revision specifies a branch with an upstream, then
	any new branch will use the named branch's upstream.`)
	delete := f.Bool("d", false, "delete the given branches")
	f.Alias("d", "delete")
	force := f.Bool("f", false, "force")
	f.Alias("f", "force")
	rev := f.String("r", "", "`rev`ision to place branches on")
	ord := branchSortOrder{key: branchSortDate, dir: descending}
	f.Var(&ord, "sort", "sort order when listing: 'name' or 'date'. May be prefixed by '-' for descending.")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	switch {
	case *delete:
		if f.NArg() == 0 {
			return usagef("must pass branch names to delete")
		}
		if *rev != "" {
			return usagef("can't pass -r for delete")
		}
		return deleteBranches(ctx, cc.git, f.Args(), *force)
	case f.NArg() == 0:
		// List
		if *force {
			return usagef("can't pass -f without branch names")
		}
		if *rev != "" {
			return usagef("can't pass -r without branch names")
		}
		return listBranches(ctx, cc, ord)
	default:
		// Create or update
		for _, b := range f.Args() {
			if strings.HasPrefix(b, "-") {
				return fmt.Errorf("invalid branch name %q", b)
			}
		}
		target := git.Head.String()
		if *rev != "" {
			target = *rev
		}
		r, err := cc.git.ParseRev(ctx, target)
		if err != nil {
			return err
		}
		var upstream string
		if b := r.Ref.Branch(); b != "" {
			cfg, err := cc.git.ReadConfig(ctx)
			if err != nil {
				return err
			}
			upstream = branchUpstream(cfg, b)
		}
		var upstreamArgs []string
		if upstream != "" {
			// TODO(soon): This should copy the configuration directly
			// instead of relying on the default tracking branch pattern.
			upstreamArgs = append(upstreamArgs, "branch", "--quiet", "--set-upstream-to="+upstream, "--", "XXX")
		}
		for i, b := range f.Args() {
			exists := false
			if len(upstreamArgs) > 0 && *force {
				// This check for existence is only necessary during -force,
				// since branch would fail otherwise. We need to check for
				// existence because we don't want to clobber upstream.
				// TODO(someday): write test that exercises this.
				_, err := cc.git.ParseRev(ctx, git.BranchRef(b).String())
				exists = err == nil
			}
			err := cc.git.NewBranch(ctx, b, git.BranchOptions{
				Overwrite:  *force,
				StartPoint: r.Commit.String(),
				Checkout:   i == 0 && *rev == "",
			})
			if err != nil {
				return fmt.Errorf("branch %q: %w", b, err)
			}
			if len(upstreamArgs) > 0 && !exists {
				upstreamArgs[len(upstreamArgs)-1] = b
				if err := cc.git.Run(ctx, upstreamArgs...); err != nil {
					return fmt.Errorf("branch %q: %w", b, err)
				}
			}
		}
	}
	return nil
}

func listBranches(ctx context.Context, cc *cmdContext, ord branchSortOrder) error {
	// Get color settings. Most errors can be ignored without impacting
	// the command output.
	var (
		currentColor []byte
		localColor   []byte
	)
	cfg, err := cc.git.ReadConfig(ctx)
	if err != nil {
		return err
	}
	colorize, err := cfg.ColorBool("color.branch", terminal.IsTerminal(cc.stdout))
	if err != nil {
		fmt.Fprintln(cc.stderr, "gg:", err)
	} else if colorize {
		currentColor, err = cfg.Color("color.branch.current", "green")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		localColor, err = cfg.Color("color.branch.local", "")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
	}

	// List branches.
	headRef, err := cc.git.HeadRef(ctx)
	if err != nil {
		return err
	}
	refs, err := cc.git.ListRefs(ctx)
	if err != nil {
		return err
	}
	commits, err := refsCommitInfo(ctx, cc.git, refs)
	if err != nil {
		return err
	}
	branches := make([]git.Ref, 0, len(refs))
	for ref := range refs {
		if ref.IsBranch() {
			branches = append(branches, ref)
		}
	}
	switch ord {
	case branchSortOrder{branchSortName, ascending}:
		sort.Slice(branches, func(i, j int) bool {
			return branches[i] < branches[j]
		})
	case branchSortOrder{branchSortName, descending}:
		sort.Slice(branches, func(i, j int) bool {
			return branches[i] > branches[j]
		})
	case branchSortOrder{branchSortDate, ascending}:
		sort.Slice(branches, func(i, j int) bool {
			return commits[refs[branches[i]]].CommitTime.Before(commits[refs[branches[j]]].CommitTime)
		})
	case branchSortOrder{branchSortDate, descending}:
		sort.Slice(branches, func(i, j int) bool {
			return commits[refs[branches[j]]].CommitTime.Before(commits[refs[branches[i]]].CommitTime)
		})
	default:
		panic("unknown sort order")
	}

	if colorize {
		if err := terminal.ResetTextStyle(cc.stdout); err != nil {
			return err
		}
	}
	for _, b := range branches {
		color, marker := localColor, ' '
		if headRef == b {
			color, marker = currentColor, '*'
		}
		commit := commits[refs[b]]
		_, err := fmt.Fprintf(cc.stdout, "%s%c %-30s %s %-20s %s\n", color, marker, b.Branch(), refs[b].Short(), commit.Author.Name, commit.Summary())
		if err != nil {
			return err
		}
		if colorize {
			if err := terminal.ResetTextStyle(cc.stdout); err != nil {
				return err
			}
		}
	}
	return nil
}

func refsCommitInfo(ctx context.Context, g *git.Git, refs map[git.Ref]git.Hash) (map[git.Hash]*git.CommitInfo, error) {
	hashes := make([]string, 0, len(refs))
	for _, c := range refs {
		present := false
		rev := c.String()
		for _, addedRev := range hashes {
			if rev == addedRev {
				present = true
				break
			}
		}
		if !present {
			hashes = append(hashes, rev)
		}
	}
	commitLog, err := g.Log(ctx, git.LogOptions{
		Revs:   hashes,
		NoWalk: true,
	})
	if err != nil {
		return nil, err
	}
	commits := make(map[git.Hash]*git.CommitInfo)
	for commitLog.Next() {
		info := commitLog.CommitInfo()
		commits[info.Hash] = info
	}
	err = commitLog.Close()
	return commits, err
}

func deleteBranches(ctx context.Context, g *git.Git, branchNames []string, force bool) error {
	branchRefs := make([]git.Ref, 0, len(branchNames))
	for _, name := range branchNames {
		r := git.BranchRef(name)
		if !r.IsValid() {
			return fmt.Errorf("invalid branch name %q", name)
		}
		branchRefs = append(branchRefs, r)
	}
	head, err := g.Head(ctx)
	if err != nil {
		return err
	}
	if head.Ref.IsValid() {
		for _, ref := range branchRefs {
			if head.Ref == ref {
				return fmt.Errorf("cannot delete checked-out branch %q", head.Ref.Branch())
			}
		}
	}
	allRefs, err := g.ListRefs(ctx)
	if err != nil {
		return err
	}
	if !force {
		for _, thisRef := range branchRefs {
			others, err := branchesContaining(ctx, g, allRefs[thisRef].String())
			if err != nil {
				return err
			}
			if len(others) <= 1 {
				return fmt.Errorf("changes in branch %q are not merged into other branches; use --force to delete", thisRef.Branch())
			}
		}
	}
	muts := make(map[git.Ref]git.RefMutation, len(branchRefs))
	for _, ref := range branchRefs {
		muts[ref] = git.DeleteRefIfMatches(allRefs[ref].String())
	}
	if err := g.MutateRefs(ctx, muts); err != nil {
		return err
	}
	return nil
}

func branchUpstream(cfg *git.Config, name string) string {
	// TODO(soon): Remove this function; the branch command should copy
	// the configuration directly.

	remote := cfg.Value("branch." + name + ".remote")
	if remote == "" {
		return ""
	}
	merge := git.Ref(cfg.Value("branch." + name + ".merge"))
	if merge == "" {
		return ""
	}
	if !merge.IsBranch() {
		return ""
	}
	return remote + "/" + merge.Branch()
}

type branchSortOrder struct {
	key string
	dir bool
}

const (
	branchSortName = "name"
	branchSortDate = "date"
)

const (
	ascending  = false
	descending = true
)

func (ord *branchSortOrder) Set(s string) error {
	if len(s) == 0 {
		return errors.New("empty sort order")
	}
	if s[0] == '-' {
		ord.dir = descending
		s = s[1:]
	} else {
		ord.dir = ascending
	}
	switch s {
	case "name", "refname":
		ord.key = branchSortName
	case "date", "creatordate", "committerdate":
		ord.key = branchSortDate
	default:
		return fmt.Errorf("unknown sort key %q", s)
	}
	return nil
}

func (ord branchSortOrder) Get() interface{} {
	return ord
}

func (ord branchSortOrder) String() string {
	if ord.dir == descending {
		return "-" + ord.key
	}
	return ord.key
}

func (ord branchSortOrder) IsBoolFlag() bool { return false }
