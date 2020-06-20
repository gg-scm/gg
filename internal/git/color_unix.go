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

//+build darwin dragonfly freebsd linux netbsd openbsd plan9 solaris

package git

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Color returns the ANSI escape sequence for the given configuration
// setting.
func (cfg *Config) Color(name string, default_ string) ([]byte, error) {
	v, ok := cfg.findLast(name)
	var desc string
	if ok {
		desc = string(v)
	} else {
		desc = default_
	}
	seq, err := parseColorDesc(desc)
	if err != nil {
		return nil, fmt.Errorf("config %s: %w", name, err)
	}
	return seq, nil
}

func parseColorDesc(val string) ([]byte, error) {
	colors := 0
	var fg, bg string
	var buf []byte
	buf = append(buf, 0x1b, '[')
	for _, word := range strings.Fields(val) {
		switch colors {
		case 0:
			if c, ok := parseColor(word, true); ok {
				fg = c
				colors = 1
				continue
			}
		case 1:
			if c, ok := parseColor(word, false); ok {
				bg = c
				colors = 2
				continue
			}
		default:
			if _, ok := parseColor(word, false); ok {
				return nil, errors.New("can specify at most foreground and background color")
			}
		}
		setattr := true
		if strings.HasPrefix(word, "no") {
			setattr = false
			word = strings.TrimPrefix(word[2:], "-")
		}
		switch word {
		case "bold":
			if len(buf) > 2 {
				buf = append(buf, ';')
			}
			if setattr {
				buf = append(buf, '1')
			} else {
				buf = append(buf, '2', '2')
			}
		case "dim":
			if len(buf) > 2 {
				buf = append(buf, ';')
			}
			if !setattr {
				buf = append(buf, '2')
			}
			buf = append(buf, '2')
		case "ul":
			if len(buf) > 2 {
				buf = append(buf, ';')
			}
			if !setattr {
				buf = append(buf, '2')
			}
			buf = append(buf, '4')
		case "blink":
			if len(buf) > 2 {
				buf = append(buf, ';')
			}
			if !setattr {
				buf = append(buf, '2')
			}
			buf = append(buf, '5')
		case "reverse":
			if len(buf) > 2 {
				buf = append(buf, ';')
			}
			if !setattr {
				buf = append(buf, '2')
			}
			buf = append(buf, '7')
		case "italic":
			if len(buf) > 2 {
				buf = append(buf, ';')
			}
			if !setattr {
				buf = append(buf, '2')
			}
			buf = append(buf, '3')
		case "strike":
			if len(buf) > 2 {
				buf = append(buf, ';')
			}
			if !setattr {
				buf = append(buf, '2')
			}
			buf = append(buf, '9')
		default:
			return nil, fmt.Errorf("unknown attribute %s", word)
		}
	}
	if fg == "" && bg == "" && len(buf) == 2 {
		return nil, nil
	}
	if fg != "" {
		if len(buf) > 2 {
			buf = append(buf, ';')
		}
		buf = append(buf, fg...)
	}
	if bg != "" {
		if len(buf) > 2 {
			buf = append(buf, ';')
		}
		buf = append(buf, bg...)
	}
	buf = append(buf, 'm')
	return buf, nil
}

func parseColor(name string, fg bool) (_ string, ok bool) {
	if strings.HasPrefix(name, "#") {
		if len(name) != 7 {
			return "", false
		}
		val, err := strconv.ParseUint(name[1:], 16, 32)
		if err != nil || val > 0xffffff {
			return "", false
		}
		if fg {
			return fmt.Sprintf("38;2;%d;%d;%d", val&0xff0000>>16, val&0x00ff00>>8, val&0x0000ff), true
		} else {
			return fmt.Sprintf("48;2;%d;%d;%d", val&0xff0000>>16, val&0x00ff00>>8, val&0x0000ff), true
		}
	}
	switch name {
	case "normal":
		return "", true
	case "black":
		if fg {
			return "30", true
		} else {
			return "40", true
		}
	case "red":
		if fg {
			return "31", true
		} else {
			return "41", true
		}
	case "green":
		if fg {
			return "32", true
		} else {
			return "42", true
		}
	case "yellow":
		if fg {
			return "33", true
		} else {
			return "43", true
		}
	case "blue":
		if fg {
			return "34", true
		} else {
			return "44", true
		}
	case "magenta":
		if fg {
			return "35", true
		} else {
			return "45", true
		}
	case "cyan":
		if fg {
			return "36", true
		} else {
			return "46", true
		}
	case "white":
		if fg {
			return "37", true
		} else {
			return "47", true
		}
	}
	if n, err := strconv.Atoi(name); err == nil && n < 8 {
		if fg {
			return fmt.Sprintf("3%d", n), true
		} else {
			return fmt.Sprintf("4%d", n), true
		}
	} else if err == nil && n < 256 {
		if fg {
			return fmt.Sprintf("38;5;%d", n), true
		} else {
			return fmt.Sprintf("48;5;%d", n), true
		}
	}
	return "", false
}

// ColorBool finds the color configuration setting is true or false.
// isTerm indicates whether the eventual output will be a terminal.
func (cfg *Config) ColorBool(name string, isTerm bool) (bool, error) {
	// Confusingly, git aliases true to "auto". false is "never".
	// "always" is true.

	v, ok := cfg.findLast(name)
	if !ok {
		if name == "color.ui" {
			return isTerm, nil
		}
		if name == "color.diff" {
			color, err := cfg.ColorBool("diff.color", isTerm)
			if err != nil {
				return false, fmt.Errorf("config %s: %w", name, err)
			}
			return color, nil
		}
		color, err := cfg.ColorBool("color.ui", isTerm)
		if err != nil {
			return false, fmt.Errorf("config %s: %w", name, err)
		}
		return color, nil
	}
	if v == nil {
		// Missing equals sign means boolean true, which means auto.
		return isTerm, nil
	}
	v = append([]byte(nil), v...)
	toLower(v)
	switch {
	case equalsString(v, "always"):
		return true, nil
	case equalsString(v, "never"):
		return false, nil
	case equalsString(v, "auto"):
		return isTerm, nil
	}
	color, ok := parseBool(v)
	if !ok {
		return false, fmt.Errorf("config %s: cannot parse %q as a bool", name, v)
	}
	return color && isTerm, nil
}
