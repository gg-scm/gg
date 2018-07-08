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

package singleclose

import (
	"fmt"
	"testing"
)

func TestCloser(t *testing.T) {
	n := new(counter)
	c := For(n)

	// First time.
	err := c.Close()
	if *n != 1 {
		t.Errorf("after Close() #0, count = %d; want 1", *n)
	}
	if err.Error() != "close #0" {
		t.Errorf("Close() #0 = %v; want close #0", err)
	}

	// Subsequent times.
	for i := 0; i < 9; i++ {
		if *n != 1 {
			t.Errorf("after Close() #%d, count = %d; want 1", i+1, n)
		}
		if err.Error() != "close #0" {
			t.Errorf("Close() #%d = %v; want close #0", i+1, err)
		}
	}
}

type counter int

func (n *counter) Close() error {
	err := fmt.Errorf("close #%d", *n)
	*n++
	return err
}
