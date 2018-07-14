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

package singleclose_test

import (
	"io/ioutil"
	"log"
	"os"

	"gg-scm.io/pkg/internal/singleclose"
)

func Example() {
	// Open a temporary file for writing.
	f, err := ioutil.TempFile("", "singleclose")
	if err != nil {
		log.Panic(err)
	}
	defer func() {
		if err := os.Remove(f.Name()); err != nil {
			log.Println(err)
		}
	}()

	// Create a *singleclose.Closer. c.Close() can be safely deferred
	// because it guarantees that f.Close() will only be called once.
	c := singleclose.For(f)
	defer c.Close()

	// Write content to the file.
	if _, err := f.WriteString("Hello, World!\n"); err != nil {
		// If this panics, f will be closed via the defer.
		log.Panic(err)
	}

	// Close explicitly and check for error.
	// When the deferred Close() is executed, it will no-op.
	if err := c.Close(); err != nil {
		log.Panic(err)
	}
}
