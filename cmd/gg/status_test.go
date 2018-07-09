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
	"bytes"
	"context"
	"testing"
)

func TestStatus(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := stageCommitTest(ctx, env, true); err != nil {
		t.Fatal(err)
	}

	out, err := env.gg(ctx, env.root, "status")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Output:\n%s", out)
	foundAdded, foundModified, foundDeleted := false, false, false
	for lineno := 1; len(out) > 0; lineno++ {
		var line []byte
		line, out = splitLine(out)
		if len(line) < 2 {
			t.Errorf("Line %d: got %q; want >3 characters for status, then space, then name", lineno, line)
			continue
		}
		if line[1] != ' ' {
			t.Errorf("Line %d: got %q; want second character to be a space", lineno, line)
		}
		switch name := string(line[2:]); name {
		case "added.txt":
			if foundAdded {
				t.Error("Duplicate for added.txt")
				continue
			}
			foundAdded = true
			if line[0] != 'A' {
				t.Errorf("Line %d: added.txt status = %q; want 'A'", lineno, line[0])
			}
		case "modified.txt":
			if foundModified {
				t.Error("Duplicate for modified.txt")
				continue
			}
			foundModified = true
			if line[0] != 'M' {
				t.Errorf("Line %d: modified.txt status = %q; want 'M'", lineno, line[0])
			}
		case "deleted.txt":
			if foundDeleted {
				t.Error("Duplicate for deleted.txt")
				continue
			}
			foundDeleted = true
			if line[0] != 'R' {
				t.Errorf("Line %d: deleted.txt status = %q; want 'R'", lineno, line[0])
			}
		default:
			t.Errorf("Line %d: got unexpected file %q", lineno, name)
		}
	}
	if !foundAdded {
		t.Error("No status line for added.txt")
	}
	if !foundModified {
		t.Error("No status line for modified.txt")
	}
	if !foundDeleted {
		t.Error("No status line for deleted.txt")
	}
}

func splitLine(b []byte) (line, rest []byte) {
	i := bytes.IndexByte(b, '\n')
	if i == -1 {
		return b, nil
	}
	return b[:i], b[i+1:]
}
