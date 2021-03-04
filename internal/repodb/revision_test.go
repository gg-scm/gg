// Copyright 2021 The gg Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//		 https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package repodb

import (
	"encoding/hex"
	"testing"
)

func TestUpperHex(t *testing.T) {
	tests := []struct {
		s    string
		want string
	}{
		{"", ""},
		{"1234", "1234"},
		{"dead", "DEAD"},
		{"DEAD", "DEAD"},
		{"main", ""},
	}
	for _, test := range tests {
		if got := upperHex(test.s); got != test.want {
			t.Errorf("upperHex(%q) = %q; want %q", test.s, got, test.want)
		}
	}
}

func TestHexRange(t *testing.T) {
	tests := []struct {
		s     string
		lower string
		upper string
	}{
		{"", "", ""},
		{"0", "00", "10"},
		{"1", "10", "20"},
		{"E", "e0", "f0"},
		{"F", "f0", ""},
		{"00", "00", "01"},
		{"01", "01", "02"},
		{"FE", "fe", "ff"},
		{"FF", "ff", ""},
		{"000", "0000", "0010"},
		{"001", "0010", "0020"},
		{"00F", "00f0", "0100"},
		{"FFE", "ffe0", "fff0"},
		{"FFF", "fff0", ""},
	}
	for _, test := range tests {
		lower, upper := hexRange(test.s)
		if hex.EncodeToString(lower) != test.lower ||
			hex.EncodeToString(upper) != test.upper {
			t.Errorf("hexRange(%q) = %q, %q; want %q, %q", test.s, lower, upper, test.lower, test.upper)
		}
	}
}
