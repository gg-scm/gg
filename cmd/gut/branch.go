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
	"strings"

	"zombiezen.com/go/gut/internal/flag"
	"zombiezen.com/go/gut/internal/gittool"
)

const branchSynopsis = "list or manage branches"

func branch(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gut branch [-d] [-f] [-r REV] [NAME [...]]", branchSynopsis+`

	Branches are references to commits to help track lines of
	development. Branches are unversioned and can be moved, renamed, and
	deleted.
	
	Creating or updating to a branch causes it to be marked as active.
	When a commit is made, the active branch will advance to the new
	commit. A plain 'gut update' will also advance an active branch, if
	possible.`)
	delete := f.Bool("d", false, "delete the given branches")
	f.Alias("d", "delete")
	force := f.Bool("f", false, "force")
	f.Alias("f", "force")
	rev := f.String("r", "", "`rev`ision to place branches on")
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
		var branchArgs []string
		branchArgs = append(branchArgs, "branch", "--delete")
		if *force {
			branchArgs = append(branchArgs, "--force")
		}
		branchArgs = append(branchArgs, "--")
		branchArgs = append(branchArgs, f.Args()...)
		if err := cc.git.Run(ctx, branchArgs...); err != nil {
			return err
		}
	case f.NArg() == 0:
		// List
		if *force {
			return usagef("can't pass -f without branch names")
		}
		if *rev != "" {
			return usagef("can't pass -r without branch names")
		}
		return cc.git.RunInteractive(ctx, "--no-pager", "branch")
	default:
		// Create or update
		for _, b := range f.Args() {
			if strings.HasPrefix(b, "-") {
				return fmt.Errorf("invalid branch name %q", b)
			}
		}
		target := "HEAD"
		if *rev != "" {
			r, err := gittool.ParseRev(ctx, cc.git, target)
			if err != nil {
				return err
			}
			target = r.CommitHex()
		}
		var branchArgs []string
		branchArgs = append(branchArgs, "branch", "--quiet")
		if *force {
			branchArgs = append(branchArgs, "--force")
		}
		branchArgs = append(branchArgs, "--", "XXX", target)
		for _, b := range f.Args() {
			branchArgs[len(branchArgs)-2] = b
			if err := cc.git.Run(ctx, branchArgs...); err != nil {
				return fmt.Errorf("branch %q: %v", b, err)
			}
		}
		if *rev == "" {
			return cc.git.Run(ctx, "symbolic-ref", "-m", "gut branch", "HEAD", "refs/heads/"+f.Arg(0))
		}
	}
	return nil
}
