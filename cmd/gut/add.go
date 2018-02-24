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

	"zombiezen.com/go/gut/internal/flag"
)

const addSynopsis = "add the specified files on the next commit"

func add(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gut add FILE [...]", addSynopsis)
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 {
		return usagef("must pass one or more files to add")
	}
	return cc.git.Run(ctx, append([]string{"add", "-N", "--"}, f.Args()...)...)
}
