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
	"io"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gitobj"
	"zombiezen.com/go/gg/internal/gittool"
)

const catSynopsis = "output the current or given revision of files"

func cat(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg cat [-r REV] FILE [...]", catSynopsis+`

	Print the specified files as they were at the given revision. If no
	revision is given, HEAD is used.`)
	r := f.String("r", gitobj.Head.String(), "print the `rev`ision")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 {
		return usagef("must pass one or more files to cat")
	}
	rev, err := gittool.ParseRev(ctx, cc.git, *r)
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

func catFile(ctx context.Context, cc *cmdContext, rev *gittool.Rev, path string) error {
	// Find path relative to top of repository. ls-files outputs files in
	// a different order than its arguments, so we have to do this one at
	// a time.
	topPath, err := cc.git.RunOneLiner(ctx, 0, "ls-tree", "-z", "--name-only", "--full-name", rev.Commit().String(), "--", ":(literal)"+path)
	if err != nil {
		return err
	}

	// Send file to stdout.
	p, err := cc.git.Start(ctx, "cat-file", "blob", rev.Commit().String()+":"+string(topPath))
	if err != nil {
		return err
	}
	_, err = io.Copy(cc.stdout, p)
	waitErr := p.Wait()
	if err != nil {
		return err
	}
	if waitErr != nil {
		return waitErr
	}
	return nil
}
