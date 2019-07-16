// Copyright 2018 The gg Authors
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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"gg-scm.io/pkg/internal/sigterm"
)

// Config is a collection of configuration settings.
type Config struct {
	data []byte
}

// ReadConfig reads all the configuration settings from Git.
func (g *Git) ReadConfig(ctx context.Context) (*Config, error) {
	c := g.command(ctx, []string{g.exe, "config", "-z", "--list"})
	stdout := new(bytes.Buffer)
	c.Stdout = &limitWriter{w: stdout, n: dataOutputLimit}
	stderr := new(bytes.Buffer)
	c.Stderr = &limitWriter{w: stdout, n: errorOutputLimit}
	if err := sigterm.Run(ctx, c); err != nil {
		return nil, commandError("read git config", err, stderr.Bytes())
	}
	cfg, err := parseConfig(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("read git config: %v", err)
	}
	return cfg, nil
}

func parseConfig(data []byte) (*Config, error) {
	cfg := &Config{
		data: data,
	}
	for off := 0; off < len(data); {
		k, _, end := splitConfigEntry(data[off:])
		if end == -1 {
			return nil, io.ErrUnexpectedEOF
		}
		toLower(k)
		off += end
	}
	return cfg, nil
}

// splitConfigEntry parses the next zero-terminated config entry, as in
// output from git config -z --list. If v == nil, then the configuration
// setting had no equals sign (usually means true for a boolean).
func splitConfigEntry(b []byte) (k, v []byte, end int) {
	kEnd := 0
	for ; kEnd < len(b); kEnd++ {
		if b[kEnd] == 0 {
			return b[:kEnd], nil, kEnd + 1
		}
		if b[kEnd] == '\n' {
			break
		}
	}
	if kEnd >= len(b) {
		return nil, nil, -1
	}
	vEnd := kEnd + 1
	for ; vEnd < len(b); vEnd++ {
		if b[vEnd] == 0 {
			break
		}
	}
	if vEnd >= len(b) {
		return nil, nil, -1
	}
	return b[:kEnd], b[kEnd+1 : vEnd], vEnd + 1
}

// CommentChar returns the value of the `core.commentChar` setting.
func (cfg *Config) CommentChar() (string, error) {
	v := cfg.Value("core.commentChar")
	if v == "" {
		return "#", nil
	}
	if v == "auto" {
		return "", errors.New("git config: core.commentChar=auto not supported")
	}
	return v, nil
}

// Value returns the string value of the configuration setting with the
// given name.
func (cfg *Config) Value(name string) string {
	v, _ := cfg.findLast(name)
	return string(v)
}

// Bool returns the boolean configuration setting with the given name.
func (cfg *Config) Bool(name string) (bool, error) {
	v, ok := cfg.findLast(name)
	if !ok {
		return false, fmt.Errorf("config %s: not found", name)
	}
	if v == nil {
		// No equals sign, which implies true.
		return true, nil
	}
	b, ok := parseBool(v)
	if !ok {
		return false, fmt.Errorf("config %s: cannot parse %q as a bool", name, v)
	}
	return b, nil
}

// Remote stores the configuration for a remote repository.
type Remote struct {
	Name     string
	FetchURL string
	Fetch    []FetchRefspec
	PushURL  string
}

// String returns the remote's name.
func (r *Remote) String() string {
	return r.Name
}

// MapFetch maps a remote fetch ref into a local ref. If there is no mapping,
// then MapFetch returns an empty Ref.
func (r *Remote) MapFetch(remote Ref) Ref {
	for _, spec := range r.Fetch {
		if local := spec.Map(remote); local != "" {
			return local
		}
	}
	return ""
}

// ListRemotes returns the names of all remotes specified in the
// configuration.
func (cfg *Config) ListRemotes() map[string]*Remote {
	remotes := make(map[string]*Remote)
	fetchURLsSet := make(map[string]bool)
	pushURLsSet := make(map[string]bool)
	remotePrefix := []byte("remote.")
	for off := 0; off < len(cfg.data); {
		k, v, end := splitConfigEntry(cfg.data[off:])
		if end == -1 {
			break
		}
		off += end
		// Looking for foo in "branch.foo.setting".
		if !bytes.HasPrefix(k, remotePrefix) {
			continue
		}
		i := bytes.LastIndexByte(k[len(remotePrefix):], '.')
		if i == -1 {
			continue
		}
		i += len(remotePrefix)

		// Get or create remote.
		name := string(k[len(remotePrefix):i])
		remote := remotes[name]
		if remote == nil {
			remote = &Remote{Name: name}
			remotes[name] = remote
		}

		// Update appropriate setting.
		// Oddly, Git seems to use the first found setting instead of
		// the last. This is verified by the test.
		switch string(k[i+1:]) {
		case "url":
			if !fetchURLsSet[name] {
				remote.FetchURL = string(v)
				fetchURLsSet[name] = true
			}
		case "pushurl":
			if !pushURLsSet[name] {
				remote.PushURL = string(v)
				pushURLsSet[name] = true
			}
		case "fetch":
			remote.Fetch = append(remote.Fetch, FetchRefspec(v))
		}
	}
	for _, remote := range remotes {
		if !pushURLsSet[remote.Name] {
			remote.PushURL = remote.FetchURL
		}
	}
	return remotes
}

func (cfg *Config) findLast(name string) (value []byte, found bool) {
	norm := []byte(name)
	toLower(norm)
	for off := 0; off < len(cfg.data); {
		k, v, end := splitConfigEntry(cfg.data[off:])
		if end == -1 {
			break
		}
		if bytes.Equal(k, norm) {
			value = v
			found = true
		}
		off += end
	}
	return
}

func parseBool(v []byte) (_ bool, ok bool) {
	if len(v) == 0 {
		return false, true
	}
	v = append([]byte(nil), v...)
	toLower(v)
	switch {
	case equalsString(v, "true") || equalsString(v, "yes") || equalsString(v, "on") || equalsString(v, "1"):
		return true, true
	case equalsString(v, "false") || equalsString(v, "no") || equalsString(v, "off") || equalsString(v, "0"):
		return false, true
	default:
		return false, false
	}
}

func toLower(b []byte) {
	// Git case-sensitivity is only used in ASCII contexts (configuration
	// setting names and booleans). Supporting Unicode could require
	// resizing the byte slice, which isn't needed.
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] = b[i] - 'A' + 'a'
		}
	}
}

func equalsString(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := range b {
		if b[i] != s[i] {
			return false
		}
	}
	return true
}
