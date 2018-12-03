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
	"fmt"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/git"
	"gg-scm.io/pkg/internal/singleclose"
)

const commitSynopsis = "commit the specified files or all outstanding changes"

func commit(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg commit [--amend] [-m MSG] [FILE [...]]", commitSynopsis+`

aliases: ci

	Commits changes to the given files into the repository. If no files
	are given, then all changes reported by `+"`gg status`"+` will be
	committed.

	Unlike Git, gg does not require you to stage your changes into the
	index. This approximates the behavior of `+"`git commit -a`"+`, but
	this command will only change the index if the commit succeeds.`)
	amend := f.Bool("amend", false, "amend the parent of the working directory")
	msg := f.String("m", "", "use text as commit `message`")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	// git does not always operate correctly on specified files when
	// running from a subdirectory (see https://github.com/zombiezen/gg/issues/10).
	// To work around, we always run commit from the top directory.
	top, err := cc.git.WorkTree(ctx)
	if err != nil {
		return err
	}
	var commitArgs []string
	commitArgs = append(commitArgs, "commit", "--quiet")
	if *amend {
		commitArgs = append(commitArgs, "--amend")
	}
	if *msg != "" {
		commitArgs = append(commitArgs, "--message="+*msg)
	}
	if f.NArg() > 0 {
		// Commit specific files.
		files, err := argsToFiles(ctx, cc.git, f.Args())
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return errors.New("arguments did not match any modified files")
		}
		commitArgs = append(commitArgs, "--")
		for _, f := range files {
			commitArgs = append(commitArgs, f.Pathspec().String())
		}
	} else if exists, err := cc.git.Query(ctx, "cat-file", "-e", "MERGE_HEAD"); err == nil && exists {
		// Merging: must not provide selective files.
		commitArgs = append(commitArgs, "-a")
	} else {
		// Commit all tracked files without modifying index.
		commitFiles, err := inferCommitFiles(ctx, cc.git)
		if err != nil {
			return err
		}
		if len(commitFiles) == 0 && !*amend {
			return errors.New("nothing changed")
		}
		commitArgs = append(commitArgs, "--")
		for _, f := range commitFiles {
			commitArgs = append(commitArgs, f.Pathspec().String())
		}
	}
	return cc.git.WithDir(top).RunInteractive(ctx, commitArgs...)
}

// argsToFiles finds the files named by the arguments.
func argsToFiles(ctx context.Context, g *git.Git, args []string) ([]git.TopPath, error) {
	statusArgs := make([]git.Pathspec, len(args))
	for i := range args {
		statusArgs[i] = git.LiteralPath(args[i])
	}
	st, err := git.Status(ctx, g, git.StatusOptions{
		Pathspecs: statusArgs,
	})
	if err != nil {
		return nil, err
	}
	stClose := singleclose.For(st)
	defer stClose.Close()
	var files []git.TopPath
	for st.Scan() {
		files = append(files, st.Entry().Name())
	}
	if err := st.Err(); err != nil {
		return nil, err
	}
	if err := stClose.Close(); err != nil {
		return nil, err
	}
	return files, nil
}

func inferCommitFiles(ctx context.Context, g *git.Git) ([]git.TopPath, error) {
	missing, missingStaged, unmerged := 0, 0, 0
	st, err := git.Status(ctx, g, git.StatusOptions{})
	if err != nil {
		return nil, err
	}
	stClose := singleclose.For(st)
	defer stClose.Close()
	var files []git.TopPath
	for st.Scan() {
		ent := st.Entry()
		switch {
		case ent.Code().IsMissing():
			missing++
			if ent.Code()[0] != ' ' {
				missingStaged++
			}
		case ent.Code().IsAdded() || ent.Code().IsModified() || ent.Code().IsRemoved() || ent.Code().IsCopied():
			// Prepend pathspec options to interpret relative to top of
			// repository and ignore globs. See gitglossary(7) for more details.
			files = append(files, ent.Name())
		case ent.Code().IsRenamed():
			files = append(files, ent.Name(), ent.From())
		case ent.Code().IsUntracked():
			// Skip
		case ent.Code().IsUnmerged():
			unmerged++
		default:
			return nil, fmt.Errorf("unhandled status: %v", ent)
		}
	}
	if err := st.Err(); err != nil {
		return nil, err
	}
	if err := stClose.Close(); err != nil {
		return nil, err
	}
	if unmerged == 1 {
		return nil, errors.New("1 unmerged file; see 'gg status'")
	}
	if unmerged > 1 {
		return nil, fmt.Errorf("%d unmerged files; see 'gg status'", unmerged)
	}
	if len(files) == 0 {
		switch missing {
		case 0:
			return nil, nil
		case 1:
			return nil, errors.New("nothing changed (1 missing file; see 'gg status')")
		default:
			return nil, fmt.Errorf("nothing changed (%d missing files; see 'gg status')", missing)
		}
	}
	if missingStaged == 1 {
		return nil, errors.New("git has staged changes for 1 missing file; see 'gg status'")
	}
	if missingStaged > 1 {
		return nil, fmt.Errorf("git has staged changes for %d missing file; see 'gg status'", missingStaged)
	}
	return files, nil
}
