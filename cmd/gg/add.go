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
	"os"
	"path/filepath"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
)

const addSynopsis = "add the specified files on the next commit"

func add(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg add FILE [...]", addSynopsis+`

	Mark files to be tracked under version control and added at the next
	commit. If `+"`add`"+` is run on a file X and X is ignored, it will be
	tracked. However, adding a directory with ignored files will not track
	the ignored files.

	`+"`add`"+` also marks merge conflicts as resolved like `+"`git add`.")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 {
		return usagef("must pass one or more files to add")
	}

	// Group arguments into files and directories.
	files := make([]string, 0, f.NArg())
	dirs := make([]string, 0, f.NArg())
	for _, a := range args {
		if !filepath.IsAbs(a) {
			a = filepath.Join(cc.dir, a)
		}
		if isdir(a) {
			dirs = append(dirs, a)
		} else {
			files = append(files, a)
		}
	}
	// Files can be explicit adds of ignored files.
	untrackedFiles, unmerged1, err := findAddFiles(ctx, cc.git, files, true)
	if err != nil {
		return err
	}
	// Directory adds should not include ignored files.
	untrackedDirs, unmerged2, err := findAddFiles(ctx, cc.git, dirs, false)
	if err != nil {
		return err
	}
	// Untracked files coming from file arguments should be marked with
	// intent to add.
	if len(untrackedFiles) > 0 {
		pathspecs := make([]git.Pathspec, 0, len(untrackedFiles))
		for _, f := range untrackedFiles {
			pathspecs = append(pathspecs, f.Pathspec())
		}
		err := cc.git.Add(ctx, pathspecs, git.AddOptions{
			IntentToAdd:    true,
			IncludeIgnored: true,
		})
		if err != nil {
			return err
		}
	}
	// Untracked files coming from directory arguments should be intent to
	// add, but no -f. A totally untracked directory will come in as a
	// single entry, which would mean -f would apply to the whole tree.
	if len(untrackedDirs) > 0 {
		pathspecs := make([]git.Pathspec, 0, len(untrackedFiles))
		for _, d := range untrackedDirs {
			pathspecs = append(pathspecs, d.Pathspec())
		}
		err := cc.git.Add(ctx, pathspecs, git.AddOptions{
			IntentToAdd: true,
		})
		if err != nil {
			return err
		}
	}
	// Unmerged files should be added in their entirety.
	if len(unmerged1)+len(unmerged2) > 0 {
		unmerged := make([]git.Pathspec, 0, len(unmerged1)+len(unmerged2))
		for _, u := range unmerged1 {
			unmerged = append(unmerged, u.Pathspec())
		}
		for _, u := range unmerged2 {
			unmerged = append(unmerged, u.Pathspec())
		}
		if err := cc.git.Add(ctx, unmerged, git.AddOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func isdir(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}

// findAddFiles finds the files described by the arguments and groups
// them based on how they should be handled by add.
func findAddFiles(ctx context.Context, g *git.Git, args []string, includeIgnored bool) (untracked, unmerged []git.TopPath, _ error) {
	if len(args) == 0 {
		return nil, nil, nil
	}
	statusArgs := make([]git.Pathspec, len(args))
	for i := range args {
		statusArgs[i] = git.LiteralPath(args[i])
	}
	st, err := g.Status(ctx, git.StatusOptions{
		Pathspecs:      statusArgs,
		IncludeIgnored: includeIgnored,
	})
	if err != nil {
		return nil, nil, err
	}
	for _, ent := range st {
		switch {
		case ent.Code.IsUntracked() || ent.Code.IsIgnored():
			untracked = append(untracked, ent.Name)
		case ent.Code.IsUnmerged():
			unmerged = append(unmerged, ent.Name)
		}
	}
	return untracked, unmerged, nil
}
