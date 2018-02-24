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

	"zombiezen.com/go/gut/internal/flag"
	"zombiezen.com/go/gut/internal/gittool"
)

const pushSynopsis = "push changes to the specified destination"

func push(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gut push [-r REV] [-d REF] [DST]", pushSynopsis)
	dstRef := f.String("d", "", "destination `ref`")
	f.Alias("d", "dest")
	rev := f.String("r", "HEAD", "source `rev`ision")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("can't pass multiple destinations")
	}
	src, err := gittool.ParseRev(ctx, cc.git, *rev)
	if err != nil {
		return err
	}
	dstRepo := f.Arg(0)
	if dstRepo == "" {
		if src.Branch() != "" {
			remote, err := gittool.Config(ctx, cc.git, "branch."+src.Branch()+".remote")
			if err == nil {
				dstRepo = remote
			} else if !gittool.IsExitError(err) {
				return err
			}
		}
		if dstRepo == "" {
			// TODO(someday): check that origin exists
			dstRepo = "origin"
		}
	}
	if *dstRef == "" {
		if src.Branch() == "" {
			return errors.New("not on a checked out branch, so can't infer destination. Use -d to specify destination branch.")
		}
		// TODO(maybe): check remote.*.push
		*dstRef = src.RefName()
	}
	return cc.git.RunInteractive(ctx, "push", "--", dstRepo, src.CommitHex()+":"+*dstRef)
}
