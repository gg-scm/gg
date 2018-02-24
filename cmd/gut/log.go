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
	"strings"

	"zombiezen.com/go/gut/internal/flag"
)

const logSynopsis = "show revision history of entire repository or files"

func log(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gut log [OPTION [...]] [FILE]", logSynopsis)
	follow := f.Bool("follow", false, "follow file history across copies and renames")
	graph := f.Bool("graph", false, "show the revision DAG")
	f.Alias("graph", "G")
	rev := f.MultiString("r", "show the specified `rev`ision or range")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("only one file allowed")
	}
	var logArgs []string
	logArgs = append(logArgs, "log", "--decorate=auto")
	if *follow {
		logArgs = append(logArgs, "--follow")
	}
	if *graph {
		logArgs = append(logArgs, "--graph")
	}
	for _, r := range *rev {
		if strings.HasPrefix(r, "-") {
			return usagef("revisions must not start with '-'")
		}
	}
	logArgs = append(logArgs, *rev...)
	logArgs = append(logArgs, "--")
	logArgs = append(logArgs, f.Args()...)
	return cc.git.RunInteractive(ctx, logArgs...)
}
