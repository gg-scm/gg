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

// Package sigterm provides graceful termination signal utilities.
package sigterm // import "gg-scm.io/pkg/internal/sigterm"

import (
	"context"
	"os"
	"os/exec"
)

// Signals returns the list of signals to listen for graceful termination.
func Signals() []os.Signal {
	return append([]os.Signal(nil), signals...)
}

// Start is like calling Start on os/exec.CommandContext but uses
// SIGTERM on Unix-based systems.
func Start(ctx context.Context, c *exec.Cmd) (wait func() error, err error) {
	waitDone := make(chan struct{})
	defer close(waitDone)
	go func() {
		select {
		case <-ctx.Done():
			terminate(c.Process)
		case <-waitDone:
		}
	}()
	if err := c.Start(); err != nil {
		return nil, err
	}
	return c.Wait, nil
}

// Run is like calling Run on os/exec.CommandContext but uses
// SIGTERM on Unix-based systems.
func Run(ctx context.Context, c *exec.Cmd) error {
	wait, err := Start(ctx, c)
	if err != nil {
		return err
	}
	return wait()
}
