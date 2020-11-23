// Copyright 2020 The gg Authors
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
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"gg-scm.io/pkg/ghdevice"
	"gg-scm.io/tool/internal/flag"
)

const gitHubLoginSynopsis = "log into GitHub"

func gitHubLogin(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg github-login", gitHubLoginSynopsis)
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() != 0 {
		return usagef("github-login takes no arguments")
	}
	token, err := gitHubDeviceFlow(ctx, cc.httpClient, loginRequested, cc.stderr)
	if err != nil {
		return err
	}
	tokenData := append([]byte(token), '\n')
	if err := cc.xdgDirs.writeSecret(gitHubTokenFilename, tokenData); err != nil {
		return fmt.Errorf("save token: %w", err)
	}
	fmt.Fprintln(cc.stderr, "Success! Your account will remembered in the future.")
	return nil
}

const gitHubTokenFilename = "github_token"

const (
	loginRequested = false
	firstTimeLogin = true
)

// gitHubDeviceFlow obtains a GitHub token using the device flow as described in
// https://docs.github.com/en/developers/apps/authorizing-oauth-apps#device-flow
func gitHubDeviceFlow(ctx context.Context, client *http.Client, mode bool, output io.Writer) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	iteration := 0
	return ghdevice.Flow(ctx, ghdevice.Options{
		ClientID:   "4f3e4a5a8231ed09c4ab",
		Scopes:     []string{"repo"},
		HTTPClient: client,
		Prompter: func(ctx context.Context, p ghdevice.Prompt) error {
			if mode == firstTimeLogin && iteration == 0 {
				fmt.Fprintf(output, "Looks like this is your first time using gg with GitHub.\n")
				fmt.Fprintf(output, "You need to authorize gg to access your GitHub account.\n\n")
			}
			if iteration > 0 {
				fmt.Fprintln(output, "The code has expired. Let's try again:")
			}
			iteration++
			fmt.Fprintf(output, "Go to %s in your browser,\n", p.VerificationURL)
			fmt.Fprintf(output, "and enter the code: %s\n", p.UserCode)
			fmt.Fprintf(output, "\nWaiting for GitHub (Ctrl-C to cancel)...\n")
			return nil
		},
	})
}
