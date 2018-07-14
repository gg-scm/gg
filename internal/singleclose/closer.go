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

// Package singleclose provides an implementation of io.Closer that
// ensures that a Close method is only called once.
package singleclose // import "gg-scm.io/pkg/internal/singleclose"

import (
	"io"
	"sync"
)

// A Closer wraps an io.Closer and ensures that Close is called only once.
type Closer struct {
	c    io.Closer
	once sync.Once
	err  error
}

// For returns a new Closer.
func For(c io.Closer) *Closer {
	return &Closer{c: c}
}

// Close closes the underlying io.Closer. After the first call, this
// does nothing but return the same error. Close is safe to call from
// multiple goroutines.
func (c *Closer) Close() error {
	c.once.Do(c.close)
	return c.err
}

func (c *Closer) close() {
	c.err = c.c.Close()
}
