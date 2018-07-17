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
	"testing"
)

func TestEditor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	const want = "I edited it!\n"
	cmd, err := env.editorCmd([]byte(want))
	if err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("[core]\neditor = %s\n", configEscape(cmd))
	if err := env.writeConfig([]byte(config)); err != nil {
		t.Fatal(err)
	}

	e := &editor{
		git:      env.git,
		tempRoot: env.rel("."),
		log: func(e error) {
			t.Error("Editor error:", e)
		},
	}
	got, err := e.open(ctx, "foo.txt", []byte("This is the initial content.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("open(...) = %q; want %q", got, want)
	}
}
