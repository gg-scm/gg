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
	"os"
	"path/filepath"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gittool"
	"zombiezen.com/go/gg/internal/singleclose"
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
	// intent to add. -f adds ignored files.
	if len(untrackedFiles) > 0 {
		gitArgs := []string{"add", "-f", "-N", "--"}
		gitArgs = append(gitArgs, untrackedFiles...)
		if err := cc.git.Run(ctx, gitArgs...); err != nil {
			return err
		}
	}
	// Untracked files coming from directory arguments should be intent to
	// add, but no -f. A totally untracked directory will come in as a
	// single entry, which would mean -f would apply to the whole tree.
	if len(untrackedDirs) > 0 {
		gitArgs := []string{"add", "-N", "--"}
		gitArgs = append(gitArgs, untrackedDirs...)
		if err := cc.git.Run(ctx, gitArgs...); err != nil {
			return err
		}
	}
	// Unmerged files should be added in their entirety.
	if len(unmerged1)+len(unmerged2) > 0 {
		gitArgs := []string{"add", "--"}
		gitArgs = append(gitArgs, unmerged1...)
		gitArgs = append(gitArgs, unmerged2...)
		if err := cc.git.Run(ctx, gitArgs...); err != nil {
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
func findAddFiles(ctx context.Context, git *gittool.Tool, args []string, includeIgnored bool) (untracked, unmerged []string, _ error) {
	if len(args) == 0 {
		return nil, nil, nil
	}
	statusArgs := make([]string, len(args))
	for i := range args {
		statusArgs[i] = ":(literal)" + args[i]
	}
	var st *gittool.StatusReader
	var err error
	if includeIgnored {
		st, err = gittool.StatusWithIgnored(ctx, git, statusArgs)
	} else {
		st, err = gittool.Status(ctx, git, statusArgs)
	}
	if err != nil {
		return nil, nil, err
	}
	stClose := singleclose.For(st)
	defer stClose.Close()
	for st.Scan() {
		ent := st.Entry()
		switch code := ent.Code(); {
		case code.IsUntracked() || code.IsIgnored():
			untracked = append(untracked, ":(top,literal)"+ent.Name())
		case code.IsUnmerged():
			unmerged = append(unmerged, ":(top,literal)"+ent.Name())
		}
	}
	if err := st.Err(); err != nil {
		return nil, nil, err
	}
	if err := stClose.Close(); err != nil {
		return nil, nil, err
	}
	return untracked, unmerged, nil
}
