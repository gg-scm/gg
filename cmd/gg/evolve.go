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
	"fmt"
	"strings"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/git"
	"gg-scm.io/pkg/internal/sigterm"
)

const evolveSynopsis = "sync with Gerrit changes in upstream"

func evolve(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg evolve [-l] [-d DST]", evolveSynopsis+`

	evolve compares HEAD with the ancestors of the given destination. If
	evolve finds any ancestors of the destination have the same Gerrit
	change ID as diverging ancestors of HEAD, it rebases the descendants
	of the latest shared change onto the corresponding commit in the
	destination.`)
	dst := f.String("d", "", "`ref` to compare with (defaults to upstream)")
	f.Alias("d", "dst")
	list := f.Bool("l", false, "list commits with match change IDs")
	f.Alias("l", "list")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	// Find our upstream
	var dstRev *git.Rev
	if *dst == "" {
		var err error
		dstRev, err = cc.git.ParseRev(ctx, "@{upstream}")
		if err != nil {
			return fmt.Errorf("no upstream found: %v", err)
		}
	} else {
		var err error
		dstRev, err = cc.git.ParseRev(ctx, *dst)
		if err != nil {
			return err
		}
	}
	// TODO(soon): Refactor into MergeBase API.
	mergeBase, err := cc.git.Run(ctx, "merge-base", dstRev.Commit.String(), git.Head.String())
	if err != nil {
		return err
	}
	mergeBase = strings.TrimSuffix(mergeBase, "\n")
	// TODO(soon): This should probably throw an error if there are merge commits.
	featureChanges, err := readChanges(ctx, cc.git, git.Head.String(), mergeBase)
	if err != nil {
		return err
	}
	upstreamChanges, err := readChanges(ctx, cc.git, dstRev.Commit.String(), mergeBase)
	if err != nil {
		return err
	}
	submitted := make(map[string]string, len(upstreamChanges))
	for _, c := range upstreamChanges {
		if c.id == "" {
			continue
		}
		submitted[c.id] = c.commitHex
	}
	if *list {
		for _, c := range featureChanges {
			if c.id == "" {
				continue
			}
			submitHex := submitted[c.id]
			if submitHex == "" {
				continue
			}
			fmt.Fprintf(cc.stdout, "< %s\n> %s\n", c.commitHex, submitHex)
		}
		return nil
	}
	last := len(featureChanges)
	for i := last - 1; i >= 0; i-- {
		c := featureChanges[i]
		if c.id == "" || submitted[c.id] == "" {
			continue
		}
		if last != i+1 {
			return fmt.Errorf("found commit %s that skips Gerrit change %s. Must manually resolve.", submitted[c.id], featureChanges[i+1].id)
		}
		last = i
	}
	if last >= len(featureChanges) {
		return nil
	}
	c := cc.git.Command(ctx, "rebase", "--onto="+submitted[featureChanges[last].id], "--no-fork-point", "--", featureChanges[last].commitHex)
	c.Stdin = cc.stdin
	c.Stdout = cc.stdout
	c.Stderr = cc.stderr
	return sigterm.Run(ctx, c)
}

type change struct {
	id        string // may be blank
	commitHex string
}

// readChanges lists the commits in head that are not base or its
// ancestors.  The commits will be in topological order: children to
// ancestors.
func readChanges(ctx context.Context, git *git.Git, head, base string) ([]change, error) {
	// TODO(soon): Refactor this into Log API.

	// Can't use %(trailers) because it's not supported on 2.7.4.
	out, err := git.Run(ctx, "log", "--date-order", "--pretty=format:%H%x00%B%x00", head, "^"+base)
	if err != nil {
		return nil, fmt.Errorf("read changes %s..%s: %v", base, head, err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	if !strings.HasSuffix(out, "\x00") {
		return nil, fmt.Errorf("read changes %s..%s: unexpected EOF", base, head)
	}
	out = out[:len(out)-1]
	fields := strings.Split(out, "\x00")
	var changes []change
	for i := 0; i < len(fields); i++ {
		hex := fields[i]
		if i > 0 {
			// log places a newline between entries.
			hex = strings.TrimPrefix(hex, "\n")
		}
		if len(hex) != 40 {
			return nil, fmt.Errorf("read changes %s..%s: parse log: invalid commit hash %q", base, head, hex)
		}

		i++
		if i >= len(fields) {
			return nil, fmt.Errorf("read changes %s..%s: parse log: unexpected EOF", base, head)
		}
		id := findChangeID(fields[i])
		changes = append(changes, change{
			id:        id,
			commitHex: hex,
		})
	}
	return changes, nil
}

func findChangeID(commitMsg string) string {
	commitMsg = strings.TrimSpace(commitMsg)
	i := strings.LastIndex(commitMsg, "\n\n")
	if i == -1 {
		return ""
	}
	trailers := strings.TrimSpace(commitMsg[i+2:])
	const prefix = "Change-Id:"
	for len(trailers) > 0 {
		var line string
		if i := strings.LastIndexByte(trailers, '\n'); i != -1 {
			line = trailers[i+1:]
		} else {
			line = trailers
		}
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
		trailers = trailers[:len(trailers)-len(line)]
		trailers = strings.TrimSuffix(trailers, "\n")
	}
	return ""
}
