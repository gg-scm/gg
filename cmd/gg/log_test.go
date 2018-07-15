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

func TestLog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	const wantMsg = "First post!!"
	h, err := dummyRev(ctx, env.git, env.root, "master", "foo.txt", wantMsg)
	if err != nil {
		t.Fatal(err)
	}

	out, err := env.gg(ctx, env.root, "log")
	if err != nil {
		t.Error(err)
	}
	if !bytes.Contains(out, []byte(h.Short())) || !bytes.Contains(out, []byte(wantMsg)) {
		t.Errorf("log does not contain either %q or %q. Output:\n%s", h.Short(), wantMsg, out)
	}
}
