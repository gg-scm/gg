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
	"io"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/git"
	"golang.org/x/xerrors"
)

const catSynopsis = "output the current or given revision of files"

func cat(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg cat [-r REV] FILE [...]", catSynopsis+`

	Print the specified files as they were at the given revision. If no
	revision is given, HEAD is used.`)
	r := f.String("r", git.Head.String(), "print the `rev`ision")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 {
		return usagef("must pass one or more files to cat")
	}
	rev, err := cc.git.ParseRev(ctx, *r)
	if err != nil {
		return err
	}
	for _, arg := range f.Args() {
		if err := catFile(ctx, cc, rev, arg); err != nil {
			return err
		}
	}
	return nil
}

func catFile(ctx context.Context, cc *cmdContext, rev *git.Rev, path string) error {
	// Find path relative to top of repository.
	paths, err := cc.git.ListTree(ctx, rev.Commit.String(), []git.Pathspec{git.LiteralPath(path)})
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return xerrors.Errorf("%s does not exist at %v", path, rev.Commit)
	}
	if len(paths) > 1 {
		return xerrors.Errorf("%s names multiple paths at %v", path, rev.Commit)
	}
	var topPath git.TopPath
	for p := range paths {
		// Guaranteed to be one iteration.
		topPath = p
	}

	// Send file to stdout.
	r, err := cc.git.Cat(ctx, rev.Commit.String(), git.TopPath(topPath))
	if err != nil {
		return err
	}
	_, err = io.Copy(cc.stdout, r)
	closeErr := r.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}
