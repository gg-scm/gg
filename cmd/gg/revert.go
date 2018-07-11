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

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gitobj"
)

const revertSynopsis = "restore files to their checkout state"

func revert(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg revert [-r REV] [--all] [FILE [...]]", revertSynopsis)
	all := f.Bool("all", false, "revert all changes when no arguments given")
	rev := f.String("r", gitobj.Head.String(), "revert to specified `rev`ision")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	var coArgs []string
	coArgs = append(coArgs, "checkout", "--quiet", *rev, "--")
	if f.NArg() > 0 {
		for _, a := range f.Args() {
			coArgs = append(coArgs, ":(literal)"+a)
		}
	} else if *all {
		coArgs = append(coArgs, ":/:")
	} else {
		return usagef("no arguments given.  Use -all to revert entire repository.")
	}
	return cc.git.Run(ctx, coArgs...)
}
