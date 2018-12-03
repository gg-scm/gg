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
	"path/filepath"
	"strings"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/git"
)

const removeSynopsis = "remove the specified files on the next commit"

func remove(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg remove [-f] [-r] [-after] FILE [...]", removeSynopsis+"\n\n"+
		"aliases: rm")
	after := f.Bool("after", false, "record delete for missing files")
	force := f.Bool("f", false, "forget added files, delete modified files")
	f.Alias("f", "force")
	recursive := f.Bool("r", false, "remove files under any directory specified")
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
		if err := verifyPresent(ctx, cc.git, f.Args()); err != nil {
			return err
		}
	}
	if *recursive {
		rmArgs = append(rmArgs, "-r")
	}
	rmArgs = append(rmArgs, "--")
	rmArgs = append(rmArgs, f.Args()...)
	return cc.git.Run(ctx, rmArgs...)
}

func verifyPresent(ctx context.Context, g *git.Git, args []string) error {
	statusArgs := make([]git.Pathspec, len(args))
	for i := range args {
		statusArgs[i] = git.LiteralPath(args[i])
	}
	st, err := git.Status(ctx, g, git.StatusOptions{
		Pathspecs: statusArgs,
	})
	if err != nil {
		return err
	}
	for st.Scan() {
		ent := st.Entry()
		if ent.Code().IsMissing() {
			st.Close()
			return fmt.Errorf("missing %s", ent.Name())
		}
	}
	if err := st.Close(); err != nil {
		return err
	}
	if err := st.Err(); err != nil {
		return err
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
