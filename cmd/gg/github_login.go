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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"gg-scm.io/tool/internal/flag"
)

const gitHubLoginSynopsis = "log into GitHub"

func gitHubLogin(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg github-login", upstreamSynopsis)
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
	const clientID = "4f3e4a5a8231ed09c4ab"
	codeData, err := postOAuth(ctx, client, "https://github.com/login/device/code", url.Values{
		"client_id": {clientID},
		"scope":     {"repo"},
	})
	if err != nil {
		return "", fmt.Errorf("github authorization flow: get device code: %w", err)
	}
	if mode == firstTimeLogin {
		fmt.Fprintf(output, "Looks like this is your first time using gg with GitHub.\n")
		fmt.Fprintf(output, "You need to authorize gg to access your GitHub account.\n\n")
	}
	fmt.Fprintf(output, "Go to %s in your browser,\n", codeData.Get("verification_uri"))
	fmt.Fprintf(output, "and enter the code: %s\n", codeData.Get("user_code"))
	fmt.Fprintf(output, "\nWaiting for GitHub (Ctrl-C to cancel)...\n")

	expiry, _ := parseSeconds(codeData.Get("expires_in"))
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(expiry))
	defer cancel()
	accessTokenRequest := url.Values{
		"client_id":   {clientID},
		"device_code": {codeData.Get("device_code")},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	interval, _ := parseSeconds(codeData.Get("interval"))
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			tokenResponse, err := postOAuth(ctx, client, "https://github.com/login/oauth/access_token", accessTokenRequest)
			if oauthErr := (*oauthError)(nil); errors.As(err, &oauthErr) {
				switch oauthErr.code {
				case "authorization_pending":
					continue
				case "slow_down":
					if oauthErr.interval > 0 {
						ticker.Reset(oauthErr.interval)
					}
					continue
				}
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return "", fmt.Errorf("github authorization flow: timed out waiting for entry")
			}
			if err != nil {
				return "", fmt.Errorf("github authorization flow: get access token: %w", err)
			}
			token := tokenResponse.Get("access_token")
			if token == "" {
				return "", fmt.Errorf("github authorization flow: get access token: server did not return an access token")
			}
			return token, nil
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				return "", fmt.Errorf("github authorization flow: timed out waiting for entry")
			}
			return "", fmt.Errorf("github authorization flow: get access token: %w", err)
		}
	}
}
