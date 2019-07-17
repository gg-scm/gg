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
	"fmt"
	"strings"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/git"
	"gg-scm.io/pkg/internal/sigterm"
	"golang.org/x/xerrors"
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
			return xerrors.Errorf("no upstream found: %w", err)
		}
	} else {
		var err error
		dstRev, err = cc.git.ParseRev(ctx, *dst)
		if err != nil {
			return err
		}
	}
	mergeBase, err := cc.git.MergeBase(ctx, dstRev.Commit.String(), git.Head.String())
	if err != nil {
		return err
	}
	// TODO(soon): This should probably throw an error if there are merge commits.
	featureChanges, err := readChanges(ctx, cc.git, git.Head.String(), mergeBase.String())
	if err != nil {
		return err
	}
	upstreamChanges, err := readChanges(ctx, cc.git, dstRev.Commit.String(), mergeBase.String())
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
			return xerrors.Errorf("found commit %s that skips Gerrit change %s. Must manually resolve.", submitted[c.id], featureChanges[i+1].id)
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
func readChanges(ctx context.Context, g *git.Git, head, base string) ([]change, error) {
	commits, err := g.Log(ctx, git.LogOptions{
		Revs: []string{head, "^" + base},
	})
	if err != nil {
		return nil, xerrors.Errorf("read changes %s..%s: %w", base, head, err)
	}
	var changes []change
	for commits.Next() {
		info := commits.CommitInfo()
		changes = append(changes, change{
			id:        findChangeID(info.Message),
			commitHex: info.Hash.String(),
		})
	}
	if err := commits.Close(); err != nil {
		return nil, xerrors.Errorf("read changes %s..%s: %w", base, head, err)
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
