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

package git

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CommitInfo stores information about a single commit.
type CommitInfo struct {
	Hash       Hash
	Parents    []Hash
	Author     User
	Committer  User
	AuthorTime time.Time
	CommitTime time.Time
	Message    string
}

// User identifies an author or committer.
type User struct {
	// Name is the user's full name.
	Name string
	// Email is the user's email address.
	Email string
}

// String returns the user information as a string in the
// form "User Name <foo@example.com>".
func (u User) String() string {
	return fmt.Sprintf("%s <%s>", u.Name, u.Email)
}

// CommitInfo obtains information about a single commit.
func (g *Git) CommitInfo(ctx context.Context, rev string) (*CommitInfo, error) {
	errPrefix := fmt.Sprintf("git log %q", rev)
	if err := validateRev(rev); err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}
	if strings.HasPrefix(rev, "^") {
		return nil, fmt.Errorf("%s: revision cannot be an exclusion", errPrefix)
	}
	if strings.Contains(rev, "..") {
		return nil, fmt.Errorf("%s: revision cannot be a range", errPrefix)
	}
	if strings.HasSuffix(rev, "^@") {
		return nil, fmt.Errorf("%s: revision cannot use parent shorthand", errPrefix)
	}

	out, err := g.run(ctx, errPrefix, "log", "--max-count=1", "-z", "--pretty=tformat:%H%x00%P%x00%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI%x00%B", rev, "--")
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(out, "\x00") {
		return nil, fmt.Errorf("%s: could not parse output", errPrefix)
	}
	fields := strings.Split(out[:len(out)-1], "\x00")
	if len(fields) != 9 {
		return nil, fmt.Errorf("%s: could not parse output", errPrefix)
	}
	hash, err := ParseHash(fields[0])
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}

	var parents []Hash
	if parentStrings := strings.Fields(fields[1]); len(parentStrings) > 0 {
		parents = make([]Hash, 0, len(parentStrings))
		for _, s := range parentStrings {
			p, err := ParseHash(s)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", errPrefix, err)
			}
			parents = append(parents, p)
		}
	}
	authorTime, err := time.Parse(time.RFC3339, fields[4])
	if err != nil {
		return nil, fmt.Errorf("%s: parse author time: %v", errPrefix, err)
	}
	commitTime, err := time.Parse(time.RFC3339, fields[7])
	if err != nil {
		return nil, fmt.Errorf("%s: parse commit time: %v", errPrefix, err)
	}
	return &CommitInfo{
		Hash:    hash,
		Parents: parents,
		Author: User{
			Name:  fields[2],
			Email: fields[3],
		},
		Committer: User{
			Name:  fields[5],
			Email: fields[6],
		},
		AuthorTime: authorTime,
		CommitTime: commitTime,
		Message:    fields[8],
	}, nil
}
