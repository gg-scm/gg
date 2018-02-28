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
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gittool"
)

const removeSynopsis = "remove the specified files on the next commit"

func remove(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg remove [-f] [-after] FILE [...]", removeSynopsis+"\n\n"+
		"aliases: rm")
	after := f.Bool("after", false, "record delete for missing files")
	force := f.Bool("f", false, "forget added files, delete modified files")
	f.Alias("f", "force")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 {
		return usagef("must pass one or more files to remove")
	}
	var rmArgs []string
	rmArgs = append(rmArgs, "rm")
	if *force {
		rmArgs = append(rmArgs, "--force")
	}
	if !*after {
		wt, err := gittool.WorkTree(ctx, cc.git)
		if err != nil {
			return err
		}
		// TODO(someday): this doesn't take into account any Git patterns.
		repoPaths := make([]string, f.NArg())
		for i, a := range f.Args() {
			var err error
			repoPaths[i], err = repoRelativePath(cc, wt, a)
			if err != nil {
				return err
			}
		}
		if err := verifyPresent(ctx, cc.git, repoPaths); err != nil {
			return err
		}
	}
	rmArgs = append(rmArgs, "--")
	rmArgs = append(rmArgs, f.Args()...)
	_ = after
	return cc.git.Run(ctx, rmArgs...)
}

func verifyPresent(ctx context.Context, git *gittool.Tool, repoPaths []string) error {
	p, err := git.Start(ctx, "status", "--porcelain=v1", "-z", "-unormal")
	if err != nil {
		return err
	}
	defer p.Wait()
	r := bufio.NewReader(p)
	for {
		ent, err := readStatusEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if !ent.isMissing() {
			continue
		}
		for _, p := range repoPaths {
			if ent.name == p {
				// TODO(maybe): convert back to original reference?
				return fmt.Errorf("missing %s", ent.name)
			}
		}
	}
	return nil
}

// repoRelativePath converts a working tree file reference to a path
// relative to the repository root.
func repoRelativePath(cc *cmdContext, worktree string, name string) (string, error) {
	a, err := filepath.EvalSymlinks(cc.abs(name))
	if err != nil {
		return "", err
	}
	prefix := worktree + string(filepath.Separator)
	if !strings.HasPrefix(a, prefix) {
		return "", fmt.Errorf("%s is not under %s", name, worktree)
	}
	return a[len(prefix):], nil
}
