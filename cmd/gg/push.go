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
	"strings"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gittool"
)

const pushSynopsis = "push changes to the specified destination"

func push(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg push [-r REV] [-d REF] [DST]", pushSynopsis+`

	When no destination repository is given, push uses the first non-
	empty configuration value of:

	1.  branch.*.pushRemote, if the source is a branch
	2.  remote.pushDefault
	3.  branch.*.remote, if the source is a branch
	4.  Otherwise, the remote called "origin" is used.

	(This is the same repository selection logic that git uses.)

	If -d is given and begins with "refs/", then it specifies the remote
	ref to update. If the argument passed to -d does not begin with
	"refs/", it is assumed to be a branch name ("refs/heads/<arg>").
	If -d is not given and the source is a ref, then the same ref name is
	used. Otherwise, push exits with a failure exit code. This differs
	from git, which will consult remote.*.push and push.default. You can
	imagine this being the most similar to push.default=current.`)
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
		var err error
		dstRepo, err = inferPushRepo(ctx, cc.git, src.Branch())
		if err != nil {
			return err
		}
	}
	if *dstRef == "" {
		if src.RefName() == "" {
			return errors.New("cannot infer destination (source is not a ref). Use -d to specify destination ref.")
		}
		*dstRef = src.RefName()
	} else if !strings.HasPrefix(*dstRef, "refs/") {
		*dstRef = "refs/heads/" + *dstRef
	}
	return cc.git.RunInteractive(ctx, "push", "--", dstRepo, src.CommitHex()+":"+*dstRef)
}

func inferPushRepo(ctx context.Context, git *gittool.Tool, branch string) (string, error) {
	if branch != "" {
		r, err := gittool.Config(ctx, git, "branch."+branch+".pushRemote")
		if err != nil {
			return "", err
		}
		if r != "" {
			return r, nil
		}
	}
	r, err := gittool.Config(ctx, git, "remote.pushDefault")
	if err != nil {
		return "", err
	}
	if r != "" {
		return r, nil
	}
	if branch != "" {
		r, err := gittool.Config(ctx, git, "branch."+branch+".remote")
		if err != nil {
			return "", err
		}
		if r != "" {
			return r, nil
		}
	}
	remotes, _ := listRemotes(ctx, git)
	if _, ok := remotes["origin"]; !ok {
		return "", errors.New("no destination given and no remote named \"origin\" found")
	}
	return "origin", nil
}
