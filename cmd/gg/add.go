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
	"zombiezen.com/go/gg/internal/gittool"
	"zombiezen.com/go/gg/internal/singleclose"
)

const addSynopsis = "add the specified files on the next commit"

func add(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg add FILE [...]", addSynopsis+`

	add also marks merge conflicts as resolved like `+"`git add`.")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 {
		return usagef("must pass one or more files to add")
	}

	normal, unmerged, err := splitUnmerged(ctx, cc.git, f.Args())
	if err != nil {
		return err
	}
	if len(normal) > 0 {
		err := cc.git.Run(ctx, append([]string{"add", "-N", "--"}, normal...)...)
		if err != nil {
			return err
		}
	}
	if len(unmerged) > 0 {
		err := cc.git.Run(ctx, append([]string{"add", "--"}, unmerged...)...)
		if err != nil {
			return err
		}
	}
	return nil
}

// splitUnmerged finds the files described by the arguments and groups
// them into normal files and unmerged files.
func splitUnmerged(ctx context.Context, git *gittool.Tool, args []string) (normal, unmerged []string, _ error) {
	statusArgs := make([]string, len(args))
	for i := range args {
		statusArgs[i] = ":(literal)" + args[i]
	}
	st, err := gittool.Status(ctx, git, statusArgs)
	if err != nil {
		return nil, nil, err
	}
	stClose := singleclose.For(st)
	defer stClose.Close()
	for st.Scan() {
		ent := st.Entry()
		if ent.Code().IsUnmerged() {
			unmerged = append(unmerged, ":(top,literal)"+ent.Name())
		} else {
			normal = append(normal, ":(top,literal)"+ent.Name())
		}
	}
	if err := st.Err(); err != nil {
		return nil, nil, err
	}
	if err := stClose.Close(); err != nil {
		return nil, nil, err
	}
	return normal, unmerged, nil
}
