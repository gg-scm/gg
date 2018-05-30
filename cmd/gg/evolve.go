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
	"bytes"
	"context"
	"errors"
	"fmt"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gitobj"
	"zombiezen.com/go/gg/internal/gittool"
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
	var dstRev *gittool.Rev
	if *dst == "" {
		var err error
		dstRev, err = gittool.ParseRev(ctx, cc.git, "@{upstream}")
		if err != nil {
			return fmt.Errorf("no upstream found: %v", err)
		}
	} else {
		var err error
		dstRev, err = gittool.ParseRev(ctx, cc.git, *dst)
		if err != nil {
			return err
		}
	}
	mergeBaseBytes, err := cc.git.RunOneLiner(ctx, '\n', "merge-base", dstRev.Commit().String(), gitobj.Head.String())
	if err != nil {
		return err
	}
	mergeBase := string(mergeBaseBytes)
	featureChanges, err := readChanges(ctx, cc.git, gitobj.Head.String(), mergeBase)
	if err != nil {
		return err
	}
	upstreamChanges, err := readChanges(ctx, cc.git, dstRev.Commit().String(), mergeBase)
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
	return cc.git.RunInteractive(ctx, "rebase", "--onto="+submitted[featureChanges[last].id], "--no-fork-point", "--", featureChanges[last].commitHex)
}

type change struct {
	id        string // may be blank
	commitHex string
}

// readChanges lists the commits in head that are not base or its
// ancestors.  The commits will be in topological order: children to
// ancestors.
func readChanges(ctx context.Context, git *gittool.Tool, head, base string) ([]change, error) {
	// TODO(soon): this should probably throw an error if there are merge commits.

	// Can't use %(trailers) because it's not supported on 2.7.4.
	p, err := git.Start(ctx, "log", "--date-order", "--pretty=format:%H%x00%B%x00", head, "^"+base)
	if err != nil {
		return nil, fmt.Errorf("read changes %s..%s: %v", base, head, err)
	}
	defer p.Wait()
	s := bufio.NewScanner(p)
	s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if len(data) == 0 {
			return 0, nil, nil
		}
		i := bytes.IndexByte(data, 0)
		if i == -1 {
			if atEOF {
				return 0, nil, errors.New("EOF without NUL byte")
			}
			return 0, nil, nil
		}
		return i + 1, data[:i], nil
	})
	var changes []change
	for i := 0; s.Scan(); i++ {
		hexBytes := s.Bytes()
		if i > 0 {
			// log places a newline between entries.
			hexBytes = bytes.TrimPrefix(hexBytes, []byte{'\n'})
		}
		hex := string(hexBytes)
		if len(hex) != 40 {
			return nil, fmt.Errorf("read changes %s..%s: parse log: invalid commit hash %q", base, head, hex)
		}
		if !s.Scan() {
			if err := s.Err(); err != nil {
				return nil, fmt.Errorf("read changes %s..%s: parse log: %v", base, head, err)
			}
			if err := p.Wait(); err != nil {
				return nil, fmt.Errorf("read changes %s..%s: %v", base, head, err)
			}
			return nil, fmt.Errorf("read changes %s..%s: parse log: unexpected EOF", base, head)
		}
		id := findChangeID(s.Bytes())
		changes = append(changes, change{
			id:        id,
			commitHex: hex,
		})
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("read changes %s..%s: parse log: %v", base, head, err)
	}
	if err := p.Wait(); err != nil {
		return nil, fmt.Errorf("read changes %s..%s: %v", base, head, err)
	}
	return changes, nil
}

func findChangeID(commitMsg []byte) string {
	commitMsg = bytes.TrimSpace(commitMsg)
	i := bytes.LastIndex(commitMsg, []byte("\n\n"))
	if i == -1 {
		return ""
	}
	trailers := bytes.TrimSpace(commitMsg[i+2:])
	prefix := []byte("Change-Id:")
	for len(trailers) > 0 {
		var line []byte
		if i := bytes.LastIndexByte(trailers, '\n'); i != -1 {
			line = trailers[i+1:]
		} else {
			line = trailers
		}
		if bytes.HasPrefix(line, prefix) {
			return string(bytes.TrimSpace(line[len(prefix):]))
		}
		trailers = trailers[:len(trailers)-len(line)]
		trailers = bytes.TrimSuffix(trailers, []byte{'\n'})
	}
	return ""
}
