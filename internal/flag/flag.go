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

// Package flag provides a command-line flag parser. Unlike the Go
// standard library flag package, it permits flags to be interspersed
// with arguments.
package flag // import "gg-scm.io/tool/internal/flag"

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// A FlagSet represents a set of defined flags. The zero value of a
// FlagSet is an empty set of flags, allows non-flag arguments to be
// interspersed with flags, and has no usage information.
type FlagSet struct {
	flags       map[string]*flag
	args        []string
	argStop     bool
	usage       string
	description string
}

type flag struct {
	name     string
	aliases  []string
	usage    string
	value    Value
	defValue string
}

// NewFlagSet returns a new, empty flag set with the specified
// intersperse behavior and usage information.
func NewFlagSet(intersperse bool, usage, description string) *FlagSet {
	f := new(FlagSet)
	f.Init(intersperse, usage, description)
	return f
}

// Init sets the argument intersperse behavior and usage information for
// a flag set. By default, the zero FlagSet allows non-flag arguments
// to be interspersed with flags.
func (f *FlagSet) Init(intersperse bool, usage, description string) {
	f.argStop = !intersperse
	f.usage = usage
	f.description = description
}

// Bool defines a bool flag with specified name, default value, and
// usage string. The return value is the address of a bool variable
// that stores the value of the flag.
func (f *FlagSet) Bool(name string, value bool, usage string) *bool {
	f.Var((*boolValue)(&value), name, usage)
	return &value
}

// BoolVar defines a bool flag with specified name, default value, and
// usage string. The argument p points to a bool variable in which to
// store the value of the flag.
func (f *FlagSet) BoolVar(p *bool, name string, value bool, usage string) {
	*p = value
	f.Var((*boolValue)(p), name, usage)
}

// Int defines an int flag with specified name, default value, and
// usage string. The return value is the address of an int variable
// that stores the value of the flag.
func (f *FlagSet) Int(name string, value int, usage string) *int {
	f.Var((*intValue)(&value), name, usage)
	return &value
}

// IntVar defines an int flag with specified name, default value, and
// usage string. The argument p points to an int variable in which to
// store the value of the flag.
func (f *FlagSet) IntVar(p *int, name string, value int, usage string) {
	*p = value
	f.Var((*intValue)(p), name, usage)
}

// String defines a string flag with specified name, default value, and
// usage string. The return value is the address of a string variable
// that stores the value of the flag.
func (f *FlagSet) String(name string, value string, usage string) *string {
	f.Var((*stringValue)(&value), name, usage)
	return &value
}

// StringVar defines a string flag with specified name, default value, and
// usage string. The argument p points to a string variable in which to
// store the value of the flag.
func (f *FlagSet) StringVar(p *string, name string, value string, usage string) {
	*p = value
	f.Var((*stringValue)(p), name, usage)
}

// MultiString defines a string flag with specified name and usage
// string. The return value is the address of a string slice variable
// that stores the value of each passed flag.
func (f *FlagSet) MultiString(name string, usage string) *[]string {
	v := new(multiStringValue)
	f.Var(v, name, usage)
	return (*[]string)(v)
}

// MultiStringVar defines a string flag with specified name and usage
// string. The argument p points to a []string variable in which to
// store the value of each passed flag.
func (f *FlagSet) MultiStringVar(p *[]string, name string, usage string) {
	f.Var((*multiStringValue)(p), name, usage)
}

// Var defines a flag with the specified name and usage string.
func (f *FlagSet) Var(value Value, name string, usage string) {
	if _, exists := f.flags[name]; exists {
		panic("flag redefined: " + name)
	}
	if f.flags == nil {
		f.flags = make(map[string]*flag)
	}
	ff := &flag{name: name, usage: usage, value: value, defValue: value.String()}
	f.flags[name] = ff
}

// Alias adds aliases for an already defined flag.
func (f *FlagSet) Alias(name string, aliases ...string) {
	ff := f.flags[name]
	if ff == nil {
		panic("flag alias for undefined: " + name)
	}
	ff.aliases = append(ff.aliases, aliases...)
	for _, a := range aliases {
		if f.flags[a] != nil {
			panic("flag redefined: " + a)
		}
		f.flags[a] = ff
	}
}

// Parse parses flag definitions from the argument list, which should
// not include the command name. Must be called after all flags in the
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
			return fmt.Errorf("invalid value %q for flag -%s: %w", val, name, err)
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

// Help prints the help.
func (f *FlagSet) Help(w io.Writer) {
	var buf bytes.Buffer
	if f.usage != "" {
		buf.WriteString("usage: ")
		buf.WriteString(f.usage)
		buf.WriteByte('\n')
	}
	if f.description != "" {
		if f.usage != "" {
			buf.WriteByte('\n')
		}
		buf.WriteString(f.description)
		buf.WriteByte('\n')
	}
	if len(f.flags) > 0 {
		if f.usage != "" || f.description != "" {
			buf.WriteByte('\n')
		}
		buf.WriteString("options:\n")
		w.Write(buf.Bytes())
		f.printDefaults(w)
	} else {
		w.Write(buf.Bytes())
	}
}

// printDefaults prints the default values of all defined command-line
// flags in the set to the given writer.
func (f *FlagSet) printDefaults(w io.Writer) {
	names := make([]string, 0, len(f.flags))
	for name, ff := range f.flags {
		if ff.name == name {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var buf bytes.Buffer
	for _, k := range names {
		ff := f.flags[k]
		buf.WriteString("  -")
		buf.WriteString(ff.name)
		name, usage := unquoteUsage(ff.value, ff.usage)
		if name != "" {
			buf.WriteByte(' ')
			buf.WriteString(name)
		}
		if len(ff.aliases) > 0 {
			aliases := append([]string(nil), ff.aliases...)
			sort.Strings(aliases)
			for _, a := range aliases {
				buf.WriteString("/-")
				buf.WriteString(a)
				if name != "" {
					buf.WriteByte(' ')
					buf.WriteString(name)
				}
			}
		}
		// Boolean flags of one ASCII letter are so common we
		// treat them specially, putting their usage on the same line.
		if buf.Len() <= 4 { // space, space, '-', 'x'.
			buf.WriteString("\t")
		} else {
			// Four spaces before the tab triggers good alignment
			// for both 4- and 8-space tab stops.
			buf.WriteString("\n    \t")
		}
		buf.WriteString(strings.Replace(usage, "\n", "\n    \t", -1))
		if ff.defValue != "" && ff.defValue != "0" && ff.defValue != "false" {
			if _, ok := ff.value.(*stringValue); ok {
				// put quotes on the value
				fmt.Fprintf(&buf, " (default %q)", ff.defValue)
			} else {
				fmt.Fprintf(&buf, " (default %v)", ff.defValue)
			}
		}
		buf.WriteByte('\n')
		w.Write(buf.Bytes())
		buf.Reset()
	}
}

func unquoteUsage(val Value, usage string) (name, usage_ string) {
	if i := strings.IndexByte(usage, '`'); i != -1 {
		if j := strings.IndexByte(usage[i+1:], '`'); j != -1 {
			j += i + 1
			name = usage[i+1 : j]
			return name, usage[:i] + name + usage[j+1:]
		}
	}
	switch val.(type) {
	case *boolValue:
		return "", usage
	case *stringValue:
		return "string", usage
	case *multiStringValue:
		return "string", usage
	}
	if val.IsBoolFlag() {
		return "", usage
	}
	return "value", usage
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

type intValue int

func (i *intValue) String() string {
	return strconv.FormatInt(int64(*i), 10)
}

func (i *intValue) Set(s string) error {
	v, err := strconv.ParseInt(s, 10, 0)
	*i = intValue(v)
	return err
}

func (i *intValue) Get() interface{} {
	return int(*i)
}

func (i *intValue) IsBoolFlag() bool {
	return false
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

type multiStringValue []string

func (s *multiStringValue) String() string {
	return strings.Join([]string(*s), " ")
}

func (s *multiStringValue) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func (s *multiStringValue) Get() interface{} {
	return []string(*s)
}

func (s *multiStringValue) IsBoolFlag() bool {
	return false
}

// IsHelp reports true if e indicates that the -help or -h is
// invoked but no such flag is defined.
func IsHelp(e error) bool {
	return e == errHelp
}

var errHelp = errors.New("flag: help requested")
