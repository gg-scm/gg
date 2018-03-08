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
	"errors"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gittool"
)

const pullSynopsis = "pull changes from the specified source"

func pull(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg pull [-u] [-r REF] [SOURCE]", pullSynopsis)
	remoteRef := f.String("r", "", "remote `ref`erence intended to be pulled")
	update := f.Bool("u", false, "update to new head if new descendants were pulled")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("can't pass multiple sources")
	}
	repo := f.Arg(0)
	var branch string
	if repo == "" || *remoteRef == "" {
		branch = currentBranch(ctx, cc)
	}
	if repo == "" {
		if branch != "" {
			var err error
			repo, err = gittool.Config(ctx, cc.git, "branch."+branch+".remote")
			if err != nil {
				return err
			}
		}
		if repo == "" {
			remotes, _ := listRemotes(ctx, cc.git)
			if _, ok := remotes["origin"]; !ok {
				return errors.New("no source given and no remote named \"origin\" found")
			}
			repo = "origin"
		}
	}
	if *remoteRef == "" {
		if branch != "" {
			*remoteRef = branch
		} else {
			*remoteRef = "HEAD"
		}
	}

	var gitArgs []string
	if *update {
		gitArgs = append(gitArgs, "pull", "--ff-only")
	} else {
		gitArgs = append(gitArgs, "fetch")
	}
	gitArgs = append(gitArgs, "--", repo, *remoteRef)
	return cc.git.Run(ctx, gitArgs...)
}

func currentBranch(ctx context.Context, cc *cmdContext) string {
	r, err := gittool.ParseRev(ctx, cc.git, "HEAD")
	if err != nil {
		return ""
	}
	return r.Branch()
}

func listRemotes(ctx context.Context, git *gittool.Tool) (map[string]struct{}, error) {
	p, err := git.Start(ctx, "remote")
	if err != nil {
		return nil, err
	}
	defer p.Wait()
	s := bufio.NewScanner(p)
	remotes := make(map[string]struct{})
	for s.Scan() {
		remotes[s.Text()] = struct{}{}
	}
	if s.Err() != nil {
		return remotes, s.Err()
	}
	err = p.Wait()
	return remotes, err
}
