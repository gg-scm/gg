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

package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
)

// Config is a collection of configuration settings.
type Config struct {
	first []byte
	next  [][]byte
}

// ReadConfig reads all the configuration settings from git.
func ReadConfig(ctx context.Context, g *Git) (*Config, error) {
	p, err := g.Start(ctx, "config", "-z", "--list")
	if err != nil {
		return nil, fmt.Errorf("read git config: %v", err)
	}
	cfg, parseErr := parseConfig(p)
	waitErr := p.Wait()
	if waitErr != nil {
		return nil, fmt.Errorf("read git config: %v", waitErr)
	}
	if parseErr != nil {
		return nil, fmt.Errorf("read git config: %v", parseErr)
	}
	return cfg, nil
}

func parseConfig(r io.Reader) (*Config, error) {
	const chunkSize = 4096
	cfg := &Config{
		first: make([]byte, 0, chunkSize),
	}
	for buf, off := &cfg.first, 0; ; {
		n, err := r.Read((*buf)[len(*buf):cap(*buf)])
		if err != nil && err != io.EOF {
			return nil, err
		}
		*buf = (*buf)[:len(*buf)+n]
		for {
			k, _, end := splitConfigEntry((*buf)[off:])
			if end == -1 {
				break
			}
			toLower(k)
			off += end
		}
		if len(*buf) == cap(*buf) {
			if off == 0 {
				return nil, errors.New("configuration setting too large")
			}
			carry := (*buf)[off:]
			*buf = (*buf)[:off]
			cfg.next = append(cfg.next, make([]byte, len(carry), chunkSize))
			buf = &cfg.next[len(cfg.next)-1]
			off = 0
			copy(*buf, carry)
		}
		if err == io.EOF {
			break
		}
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

func (cfg *Config) findLast(name string) (value []byte, found bool) {
	norm := []byte(name)
	toLower(norm)
	for i := 0; i < len(cfg.first); {
		k, v, end := splitConfigEntry(cfg.first[i:])
		if end == -1 {
			break
		}
		if bytes.Equal(k, norm) {
			value = v
			found = true
		}
		i += end
	}
	for _, buf := range cfg.next {
		for i := 0; i < len(cfg.first); {
			k, v, end := splitConfigEntry(buf[i:])
			if end == -1 {
				break
			}
			if bytes.Equal(k, norm) {
				value = v
				found = true
			}
			i += end
		}
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
