// Copyright 2020 The gg Authors
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
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
)

const addRemoveSynopsis = "add all new files, delete all missing files"

func addRemove(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg addremove FILE [...]", addRemoveSynopsis)
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	var pathspecs []git.Pathspec
	var doNotIgnore []git.Pathspec
	if len(args) == 0 {
		root, err := cc.git.WorkTree(ctx)
		if err != nil {
			return err
		}
		pathspecs = []git.Pathspec{git.LiteralPath(root)}
	} else {
		for _, a := range args {
			if info, err := os.Stat(cc.abs(a)); err == nil && !info.IsDir() {
				doNotIgnore = append(doNotIgnore, git.LiteralPath(a))
			} else {
				pathspecs = append(pathspecs, git.LiteralPath(a))
			}
		}
	}
	err1 := cc.git.Add(ctx, pathspecs, git.AddOptions{
		IntentToAdd: true,
	})
	err2 := cc.git.Add(ctx, doNotIgnore, git.AddOptions{
		IntentToAdd:    true,
		IncludeIgnored: true,
	})
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}
