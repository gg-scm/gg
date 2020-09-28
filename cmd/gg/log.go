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
	"strings"

	"gg-scm.io/tool/internal/flag"
)

const logSynopsis = "show revision history of entire repository or files"

func log(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg log [OPTION [...]] [FILE]", logSynopsis+`

aliases: history`)
	follow := f.Bool("follow", false, "follow file history across copies and renames")
	followFirst := f.Bool("follow-first", false, "only follow the first parent of merge commits")
	graph := f.Bool("graph", false, "show the revision DAG")
	f.Alias("graph", "G")
	rev := f.MultiString("r", "show the specified `rev`ision or range")
	reverse := f.Bool("reverse", false, "reverse order of commits")
	stat := f.Bool("stat", false, "include diffstat-style summary of each commit")
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
	logArgs = append(logArgs, "log", "--decorate=auto", "--date-order")
	if *follow {
		logArgs = append(logArgs, "--follow")
	}
	if *followFirst {
		logArgs = append(logArgs, "--first-parent")
	}
	if *graph {
		logArgs = append(logArgs, "--graph")
	}
	if *reverse {
		logArgs = append(logArgs, "--reverse")
	}
	if *stat {
		logArgs = append(logArgs, "--stat")
	}
	for _, r := range *rev {
		if strings.HasPrefix(r, "-") {
			return usagef("revisions must not start with '-'")
		}
	}
	if len(*rev) == 0 {
		logArgs = append(logArgs, "--all")
	} else {
		logArgs = append(logArgs, *rev...)
	}
	logArgs = append(logArgs, "--")
	logArgs = append(logArgs, f.Args()...)
	return cc.interactiveGit(ctx, logArgs...)
}
