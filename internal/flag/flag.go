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

// Package flag provides a command-line flag parser.  Unlike the Go
// standard library flag package, it permits flags to be interspersed
// with arguments.
package flag

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// A FlagSet represents a set of defined flags.  The zero value of a
// FlagSet is an empty set of flags and allows non-flag arguments to be
// interspersed with flags.
type FlagSet struct {
	flags   map[string]*flag
	args    []string
	argStop bool
}

type flag struct {
	name     string
	usage    string
	value    Value
	defValue string
}

// NewFlagSet returns a new, empty flag set with the specified
// intersperse behavior.
func NewFlagSet(intersperse bool) *FlagSet {
	f := new(FlagSet)
	f.Init(intersperse)
	return f
}

// Init sets the argument intersperse behavior for a flag set.  By
// default, the zero FlagSet allows non-flag arguments to be
// interspersed with flags.
func (f *FlagSet) Init(intersperse bool) {
	f.argStop = !intersperse
}

// Bool defines a bool flag with specified name, default value, and
// usage string.  The return value is the address of a bool variable
// that stores the value of the flag.
func (f *FlagSet) Bool(name string, value bool, usage string) *bool {
	f.Var((*boolValue)(&value), name, usage)
	return &value
}

// String defines a string flag with specified name, default value, and
// usage string.  The return value is the address of a string variable
// that stores the value of the flag.
func (f *FlagSet) String(name string, value string, usage string) *string {
	f.Var((*stringValue)(&value), name, usage)
	return &value
}

// Var defines a flag with the specified name and usage string.
func (f *FlagSet) Var(value Value, name string, usage string) {
	if _, exists := f.flags[name]; exists {
		panic("flag redefined: " + name)
	}
	if f.flags == nil {
		f.flags = make(map[string]*flag)
	}
	ff := &flag{name, usage, value, value.String()}
	f.flags[name] = ff
}

// Parse parses flag definitions from the argument list, which should
// not include the command name.  Must be called after all flags in the
// FlagSet are defined and before flags are accessed by the program.
// The returned error can be tested with IsHelpUndefined if -help or -h
// were set but not defined.
func (f *FlagSet) Parse(arguments []string) error {
	f.args = make([]string, 0, len(arguments))
	i := 0
flags:
	for ; i < len(arguments); i++ {
		a := arguments[i]
		var name, val string
		var hasval bool
		switch {
		case a == "--":
			i++
			break flags
		case strings.HasPrefix(a, "--"):
			name, val, hasval = split(a[2:])
		case a == "-":
			f.args = append(f.args, a)
			continue
		case strings.HasPrefix(a, "-"):
			name, val, hasval = split(a[1:])
		default:
			if f.argStop {
				break flags
			}
			f.args = append(f.args, a)
			continue
		}
		ff := f.flags[name]
		if ff == nil {
			if name == "h" || name == "help" {
				return errHelp
			}
			return fmt.Errorf("flag provided but not defined: -%s", name)
		}
		if !hasval {
			if ff.value.IsBoolFlag() {
				val = "true"
			} else if i+1 >= len(arguments) {
				return fmt.Errorf("flag needs an argument: -%s", name)
			} else {
				i++
				val = arguments[i]
			}
		}
		if err := ff.value.Set(val); err != nil {
			return fmt.Errorf("invalid value %q for flag -%s: %v", val, name, err)
		}
	}
	f.args = append(f.args, arguments[i:]...)
	return nil
}

func split(f string) (name, value string, hasValue bool) {
	i := strings.IndexByte(f, '=')
	if i == -1 {
		return f, "", false
	}
	return f[:i], f[i+1:], true
}

// Args returns the non-flag arguments.
func (f *FlagSet) Args() []string {
	return f.args[:len(f.args):len(f.args)]
}

// NArg returns the number of non-flag arguments.
func (f *FlagSet) NArg() int {
	return len(f.args)
}

// Arg returns the i'th argument. Arg(0) is the first non-flag argument.
// Arg returns an empty string if the requested element does not exist.
func (f *FlagSet) Arg(i int) string {
	if i < 0 || i >= len(f.args) {
		return ""
	}
	return f.args[i]
}

// Value is the interface to the dynamic value stored in a flag.
type Value interface {
	// String presents the current value as a string.
	String() string

	// Set is called once, in command line order, for each flag present.
	Set(string) error

	// Get returns the contents of the Value.
	Get() interface{}

	// If IsBoolFlag returns true, then the command-line parser makes
	// -name equivalent to -name=true rather than using the next
	// command-line argument.
	IsBoolFlag() bool
}

type boolValue bool

func (b *boolValue) String() string {
	return strconv.FormatBool(bool(*b))
}

func (b *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	*b = boolValue(v)
	return err
}

func (b *boolValue) Get() interface{} {
	return bool(*b)
}

func (b *boolValue) IsBoolFlag() bool {
	return true
}

type stringValue string

func (s *stringValue) String() string {
	return string(*s)
}

func (s *stringValue) Set(v string) error {
	*s = stringValue(v)
	return nil
}

func (s *stringValue) Get() interface{} {
	return string(*s)
}

func (s *stringValue) IsBoolFlag() bool {
	return false
}

// IsHelpUndefined reports true if e indicates that the -help or -h is
// invoked but no such flag is defined.
func IsHelpUndefined(e error) bool {
	return e == errHelp
}

var errHelp = errors.New("flag: help requested")
