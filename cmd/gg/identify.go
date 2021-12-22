// Copyright 2019 The gg Authors
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
	"bytes"
	"context"
	"sort"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
)

const identifySynopsis = "identify the working directory or specified revision"

func identify(ctx context.Context, cc *cmdContext, args []string) (err error) {
	f := flag.NewFlagSet(true, "gg identify [-r REV]", identifySynopsis+`

aliases: id

	Print a summary of the revision or working directory if no revision
	was provided. The revision's hash identifier is printed, followed by
	a "+" if the working copy is being summarized and there are
	uncommitted changes, a list of branches it is the tip of, and a list
	of tags.`)
	revFlag := f.String("r", "HEAD", "identify the specified `rev`ision")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 0 {
		return usagef("identify takes no arguments")
	}

	rev, err := cc.git.ParseRev(ctx, *revFlag)
	if err != nil {
		return err
	}

	hasChanges := false
	if *revFlag == "HEAD" || *revFlag == "@" {
		status, err := cc.git.Status(ctx, git.StatusOptions{})
		if err != nil {
			return err
		}
		for _, ent := range status {
			if !(ent.Code.IsUntracked() || ent.Code.IsIgnored()) {
				hasChanges = true
				break
			}
		}
	}

	refs, err := cc.git.ListRefs(ctx)
	if err != nil {
		return err
	}
	var branchNames []string
	var tagNames []string
	for ref, hash := range refs {
		if rev.Commit != hash {
			continue
		}
		if b := ref.Branch(); b != "" {
			branchNames = append(branchNames, b)
			continue
		}
		if t := ref.Tag(); t != "" {
			tagNames = append(tagNames, t)
			continue
		}
	}
	sort.Strings(branchNames)
	sort.Strings(tagNames)

	out := new(bytes.Buffer)
	out.WriteString(rev.Commit.String())
	if hasChanges {
		out.WriteByte('+')
	}
	for _, name := range branchNames {
		out.WriteByte(' ')
		out.WriteString(name)
	}
	for _, name := range tagNames {
		out.WriteByte(' ')
		out.WriteString(name)
	}
	out.WriteByte('\n')
	_, err = cc.stdout.Write(out.Bytes())
	return err
}
