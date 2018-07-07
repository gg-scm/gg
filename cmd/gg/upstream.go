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

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gitobj"
	"zombiezen.com/go/gg/internal/gittool"
)

const upstreamSynopsis = "query or set upstream branch"

func upstream(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg upstream [-b BRANCH] [REF]", upstreamSynopsis+`

	If no positional arguments are given, the branch's upstream branch is
	printed to stdout (defaulting to the current branch if none given).

	If a ref argument is given, then the branch's upstream branch
	(specified by `+"`branch.*.remote`"+` and `+"`branch.*.merge`"+` configuration
	settings) will be set to the given value.`)
	branch := f.String("b", "", "`branch` to query or modify")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("cannot set multiple upstreams")
	}
	if *branch == "" {
		rev, err := gittool.ParseRev(ctx, cc.git, gitobj.Head.String())
		if err != nil {
			return err
		}
		*branch = rev.Ref().Branch()
		if *branch == "" {
			return errors.New("no branch currently checked out; please specify branch with -b")
		}
	}
	if f.Arg(0) == "" {
		rev, err := gittool.ParseRev(ctx, cc.git, *branch+"@{upstream}")
		if err != nil {
			return err
		}
		fmt.Fprintln(cc.stdout, rev.Ref())
		return nil
	}
	return cc.git.RunInteractive(ctx, "branch", "--set-upstream-to="+f.Arg(0), "--", *branch)
}
