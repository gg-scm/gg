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
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gittool"
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

	topBytes, err := cc.git.RunOneLiner(ctx, '\n', "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	top := string(topBytes)
	pathspecs := append([]string(nil), f.Args()...)
	if err := toPathspecs(cc.dir, top, pathspecs); err != nil {
		return err
	}
	normal, unmerged, err := splitUnmerged(ctx, cc.git, pathspecs)
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

// splitUnmerged filters out the unmerged files from a list of pathspecs.
// The pathspecs slice will be mutated in-place.
func splitUnmerged(ctx context.Context, git *gittool.Tool, pathspecs []string) (normal, unmerged []string, _ error) {
	const prefix = ":(top,literal)"
	for _, spec := range pathspecs {
		// TODO(someday): It would be better to find a way to map git status
		// entries back to arguments provided.  This depends far too much on
		// the output of toPathspecs.
		if !strings.HasPrefix(spec, prefix) {
			return nil, nil, fmt.Errorf("file %q is outside the working copy; cannot be added", spec[len(":(literal)"):])
		}
	}
	statusArgs := []string{"status", "--porcelain", "-z", "-unormal", "--"}
	statusArgs = append(statusArgs, pathspecs...)
	p, err := git.Start(ctx, statusArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer p.Wait()
	r := bufio.NewReader(p)
	for {
		ent, err := readStatusEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			if waitErr := p.Wait(); waitErr != nil {
				return nil, nil, waitErr
			}
			return nil, nil, err
		}
		if !ent.isUnmerged() {
			continue
		}
		unmerged = append(unmerged, prefix+ent.name)
		// Filter out from pathspecs
		n := 0
		for i := range pathspecs {
			if pathspecs[i][len(prefix):] != ent.name {
				pathspecs[n] = pathspecs[i]
				n++
			}
		}
		pathspecs = pathspecs[:n]
	}
	err = p.Wait()
	return pathspecs, unmerged, err
}
