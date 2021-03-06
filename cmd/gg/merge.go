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

	"gg-scm.io/tool/internal/flag"
)

const mergeSynopsis = "merge another revision into working directory"

func merge(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg merge [[-r] REV]", mergeSynopsis)
	rev := f.String("r", "", "`rev`ision to merge")
	abort := f.Bool("abort", false, "abort the ongoing merge")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if *abort {
		if f.NArg() != 0 || *rev != "" {
			return usagef("cannot specify revision with --abort")
		}
		return cc.git.AbortMerge(ctx)
	}
	if f.NArg() > 1 || (f.Arg(0) != "" && *rev != "") {
		return usagef("must pass at most one revision to merge")
	}
	if *rev == "" {
		*rev = f.Arg(0)
	}
	if *rev == "" {
		*rev = "@{upstream}"
	}
	if err := cc.git.Merge(ctx, []string{*rev}); err != nil {
		return err
	}
	return nil
}
