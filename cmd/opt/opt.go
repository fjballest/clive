/*
	command line options, in the unix style and working on any
	argv[] array.

	The functions that define new flags operate in the same way:
	They define a new flag and, if a pointer is given, the value pointed
	is set when the flag is present in the command line.
	They call Fatal if a flag is defined twice.

	Flags should be defined by the same process, there is no mutex.
*/
package opt

import (
	"bytes"
	"clive/dbg"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type def  {
	name, help string
	valp       interface{}
	argname    string
}

// A set of command line options
type Flags  {
	Argv0       string // program name from the last call to Parse
	usage       string // usage string w/o program name
	defs        map[string]*def
	plus, minus *def // defs for +int -int
}

// Use Counter as the value for counting flags, which are bool flags
// that can be repeated. Their value is the number of repetitions.
type Counter int

// Use Octal as the value for an int flag expressed in octal.
type Octal int

// Use Hexa as the value for an int flag expressed in hexa.
type Hexa int

// time formats for option arguments
var tfmts = []string{
	"Jan 2",
	"01/02",
	"01/02/06",
	"01/02/2006",
	"Jan 2 2006",
	"2006/0102",
	"15:04:05",
	"15:04",
	"Jan 2 15:04",
	"3pm",
	"3:04pm",
	"Jan 2 3pm",
	"Jan 2 3:04pm",
	"01/02 15:04",
	"01/02/06 15:04",
	"01/02/2006 15:04",
	"2006/0102 15:04",
	"2006-01-02",
	"2006-01-02 3pm",
	"2006-01-02 3:04pm",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
}

// Create a new set of command line options.
// Default values are to be set by the caller before processing the options.
// The functions define new flags and, if a pointer is given, it is set to the
// option value when the option is set in the command line.
func New(usage string) *Flags {
	return &Flags{
		defs:  map[string]*def{},
		usage: usage,
	}
}

func (f *Flags) optUsage(order []string) string {
	flgs := ""
	for _, kn := range order {
		k := f.defs[kn]
		switch k.valp.(type) {
		case *bool:
			if utf8.RuneCountInString(k.name) == 1 {
				flgs += k.name
			}
		}
	}
	var buf bytes.Buffer
	if flgs != "" {
		fmt.Fprintf(&buf, "[-%s]", flgs)
	}
	flgs = ""
	for _, kn := range order {
		k := f.defs[kn]
		switch k.valp.(type) {
		case *Counter:
			if utf8.RuneCountInString(k.name) == 1 {
				flgs += k.name
			}
		}
	}
	if flgs != "" {
		fmt.Fprintf(&buf, " {-%s}", flgs)
	}
	if f.plus != nil {
		fmt.Fprintf(&buf, " [+%s]", f.plus.name)
	}
	if f.minus != nil {
		fmt.Fprintf(&buf, " [-%s]", f.plus.name)
	}
	for _, kn := range order {
		k := f.defs[kn]
		switch k.valp.(type) {
		case *bool, *Counter:
			if utf8.RuneCountInString(k.name) == 1 {
				continue
			}
		}
		switch k.valp.(type) {
		case *bool:
			fmt.Fprintf(&buf, " [-%s]", k.name)
			continue
		case *Counter:
			fmt.Fprintf(&buf, " {-%s}", k.name)
			continue
		}
		arg := "arg"
		if i := strings.Index(k.help, ":"); i >= 0 {
			arg = k.help[:i]
		}
		switch k.valp.(type) {
		case *[]string:
			fmt.Fprintf(&buf, " {-%s %s}", k.name, arg)
		default:
			fmt.Fprintf(&buf, " [-%s %s]", k.name, arg)
		}
	}
	return buf.String()
}

// Print to stderr a description of the usage.
func (f *Flags) Usage(w io.Writer) {
	if f.Argv0 == "" {
		f.Argv0 = os.Args[0]
	}
	ks := []string{}
	for k := range f.defs {
		ks = append(ks, k)
	}
	sort.Sort(sort.StringSlice(ks))
	opts := f.optUsage(ks)
	fmt.Fprintf(w, "usage: %s %s %s\n", f.Argv0, opts, f.usage)
	if f.plus != nil {
		sep := ""
		if !strings.Contains(f.plus.help, ":") {
			sep = ":"
		}
		fmt.Fprintf(w, "\t+%s%s %s\n", f.plus.name, sep, f.plus.help)
	}
	if f.minus != nil {
		sep := ""
		if !strings.Contains(f.minus.help, ":") {
			sep = ":"
		}
		fmt.Fprintf(w, "\t-%s %s\n", f.minus.name, sep, f.minus.help)
	}
	for _, k := range ks {
		def := f.defs[k]
		sep := ""
		if !strings.Contains(def.help, ":") {
			sep = ":"
		}
		switch def.valp.(type) {
		case *Counter:
			fmt.Fprintf(w, "\t-%s%s %s\n", def.name, sep, def.help)
			fmt.Fprintf(w, "\t\tcan be repeated\n")
		default:
			fmt.Fprintf(w, "\t-%s%s %s\n", def.name, sep, def.help)
		}
	}
}

// Define a new flag with the given name and usage.
// valuep must be a pointer to the argument type and will be set to
// the command line flag value if the flag is found.
// Known types are bool, int, Counter, Octal, Hexa, int64, uint64, string,
// float64, time.Duration, time.Time, and []string.
// []string is a string option that may be repeated.
// The time formats understood are
//	"01/02"
//	"01/02/06"
//	"01/02/2006"
//	"2006/0102"
//	"15:04:05"
//	"15:04"
//	"3pm"
//	"3:04pm"
//	"01/02 15:04"
//	"01/02/06 15:04"
//	"01/02/2006 15:04"
//	"2006/0102 15:04 "
//
// If the name is "+..." or "-..." and vp is *int, then it is understood as
// a request to accept +number or -number as an argument.
//
// The help string should just describe the flag for flags with no
// argument, and should be something like "dir: do this with dir"
// if the flag accepts a "dir" argument. This convention is used to generate
// a good usage diagnostic.
func (f *Flags) NewFlag(name, help string, vp interface{}) {
	if vp == nil {
		dbg.Fatal("flag %s: nil value", name)
	}
	if len(name) == 0 {
		dbg.Fatal("empty flag name")
	}
	if f.defs[name] != nil {
		dbg.Fatal("flag %s redefined", name)
	}
	aname := ""
	if i := strings.Index(help, ":"); i > 0 {
		aname = help[:i]
	}
	switch vp.(type) {
	case *bool:

	case *int:
		if aname == "" {
			aname = "num"
		}
		if name[0] == '+' {
			if f.plus != nil {
				dbg.Fatal("flag +number redefined")
			}
			f.plus = &def{name: name, help: help, valp: vp, argname: aname}
			return
		}
		if name[0] == '-' {
			if f.minus != nil {
				dbg.Fatal("flag -number redefined")
			}
			f.minus = &def{name: name, help: help, valp: vp, argname: aname}
			return
		}
	case *Counter:
	case *Octal, *Hexa, *int64, *uint64, *float64:
		if aname == "" {
			aname = "num"
		}
	case *string, *[]string:
		if aname == "" {
			aname = "str"
		}
	case *time.Duration:
		if aname == "" {
			aname = "ival"
		}
	case *time.Time:
		if aname == "" {
			aname = "time"
		}
	default:
		dbg.Fatal("flag %s: unknown flag type", name)
	}
	if name[0]=='+' || name[0]=='-' {
		dbg.Fatal("name '±...' is only for *int")
	}
	f.defs[name] = &def{name: name, help: help, valp: vp, argname: aname}
}

// Parse argv for the the flags and return the resulting argument vector w/o flags.
// The first entry in argv is the program name.
// A "--" argument terminates the options.
// A "-?" argument fails with a "usage" error
func (f *Flags) Parse(argv []string) ([]string, error) {
	var err error
	if len(argv) == 0 {
		return nil, errors.New("short argv")
	}
	f.Argv0 = argv[0]
	argv = argv[1:]
	args := make([]string, len(argv))
	copy(args, argv)
Loop:
	for len(args)>0 && len(args[0])>0 && (args[0][0]=='-' || args[0][0]=='+') {
		if args[0] == "-?" {
			return nil, errors.New("usage")
		}
		if f.plus!=nil && args[0][0]=='+' {
			args, err = f.plus.parsePlus(args)
			if err != nil {
				return nil, err
			}
			continue Loop
		}
		isdigit := len(args[0])>1 && args[0][1]>='0' && args[0][1]<='9'
		if f.minus!=nil && isdigit {
			args, err = f.minus.parseMinus(args)
			if err != nil {
				return nil, err
			}
			continue Loop
		}
		args[0] = args[0][1:] // drop "-"
		name := args[0]
		if name == "" {
			return nil, errors.New("'-' supplied without option name")
		}
		if name == "-" {
			return args[1:], nil
		}
		// try full-names first
		for n, def := range f.defs {
			if name == n {
				args, err = def.parse(args)
				if err != nil {
					return nil, err
				}
				continue Loop
			}
		}
		// try combined flags now
		for len(args)>0 && len(args[0])>0 {
			r, nr := utf8.DecodeRuneInString(args[0])
			name = args[0][:nr]
			for n, def := range f.defs {
				if name == n {
					args, err = def.parse(args)
					if err != nil {
						return nil, err
					}
					continue Loop
				}
			}
			// no flag defined for the rune
			return nil, fmt.Errorf("unknown option '%c'", r)
		}
	}
	return args, err
}

func optArg(argv []string) ([]string, string, error) {
	if len(argv)>0 && len(argv[0])==0 {
		argv = argv[1:]
	}
	if len(argv) == 0 {
		return argv, "", errors.New("no argument")
	}
	return argv[1:], argv[0], nil
}

// argv is 1st opt. argument with the + or - present.
func (d def) parsePlus(argv []string) ([]string, error) {
	if d.valp == nil {
		return nil, fmt.Errorf("undefined value for option '+%s'", d.name)
	}
	arg := argv[0][1:]
	v, err := strconv.ParseInt(arg, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("option '+%s': wrong value '%s'", d.name, arg)
	}
	switch vp := d.valp.(type) {
	case *int:
		*vp = int(v)
		return argv[1:], nil
	default:
		return nil, fmt.Errorf("+%s:bug for type %T", d.name, vp)
	}
}

// argv is 1st opt. argument with the + or - present.
func (d def) parseMinus(argv []string) ([]string, error) {
	if d.valp == nil {
		return nil, fmt.Errorf("undefined value for option '-%s'", d.name)
	}
	arg := argv[0][1:]
	v, err := strconv.ParseInt(arg, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("option '-%s': wrong value '%s'", d.name, arg)
	}
	switch vp := d.valp.(type) {
	case *int:
		*vp = -int(v)
		return argv[1:], nil
	default:
		return nil, fmt.Errorf("-%s: bug for type %T", d.name, vp)
	}
}

// argv has the 1st -option argument, with the - and the option name removed.
// Set the value for the option, if any, removing any argument used or return an error.
// Changes argv even on errors.
func (d *def) parse(argv []string) ([]string, error) {
	if d.valp == nil {
		return nil, fmt.Errorf("undefined value for option '%s'", d.name)
	}
	argv[0] = argv[0][len(d.name):]
	switch vp := d.valp.(type) {
	case *bool:
		*vp = true
		if len(argv[0]) == 0 {
			argv = argv[1:]
		} else { // put back the "-" for the next flag
			argv[0] = "-" + argv[0]
		}
	case *Counter:
		*vp = *vp + 1
		if len(argv[0]) == 0 {
			argv = argv[1:]
		} else { // put back the "-" for the next flag
			argv[0] = "-" + argv[0]
		}
	case *Octal:
		nargv, arg, err := optArg(argv)
		if err != nil {
			return nil, fmt.Errorf("option '%s': %s", d.name, err)
		}
		argv = nargv
		v, err := strconv.ParseInt(arg, 8, 0)
		if err != nil {
			return nil, fmt.Errorf("option '%s': wrong value '%s'", d.name, arg)
		}
		*vp = Octal(v)
	case *Hexa:
		nargv, arg, err := optArg(argv)
		if err != nil {
			return nil, fmt.Errorf("option '%s': %s", d.name, err)
		}
		argv = nargv
		v, err := strconv.ParseInt(arg, 16, 0)
		if err != nil {
			return nil, fmt.Errorf("option '%s': wrong value '%s'", d.name, arg)
		}
		*vp = Hexa(v)
	case *int:
		nargv, arg, err := optArg(argv)
		if err != nil {
			return nil, fmt.Errorf("option '%s': %s", d.name, err)
		}
		argv = nargv
		v, err := strconv.ParseInt(arg, 0, 0)
		if err != nil {
			return nil, fmt.Errorf("option '%s': wrong value '%s'", d.name, arg)
		}
		*vp = int(v)
	case *int64:
		nargv, arg, err := optArg(argv)
		if err != nil {
			return nil, fmt.Errorf("option '%s': %s", d.name, err)
		}
		argv = nargv
		*vp, err = strconv.ParseInt(arg, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("option '%s': wrong value '%s'", d.name, arg)
		}
	case *uint64:
		nargv, arg, err := optArg(argv)
		if err != nil {
			return nil, fmt.Errorf("option '%s': %s", d.name, err)
		}
		argv = nargv
		*vp, err = strconv.ParseUint(arg, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("option '%s': wrong value '%s'", d.name, arg)
		}
	case *string:
		nargv, arg, err := optArg(argv)
		if err != nil {
			return nil, fmt.Errorf("option '%s': %s", d.name, err)
		}
		argv, *vp = nargv, arg
	case *[]string:
		nargv, arg, err := optArg(argv)
		if err != nil {
			return nil, fmt.Errorf("option '%s': %s", d.name, err)
		}
		argv, *vp = nargv, append(*vp, arg)
	case *float64:
		nargv, arg, err := optArg(argv)
		if err != nil {
			return nil, fmt.Errorf("option '%s': %s", d.name, err)
		}
		argv = nargv
		*vp, err = strconv.ParseFloat(arg, 64)
		if err != nil {
			return nil, fmt.Errorf("option '%s': wrong value '%s'", d.name, arg)
		}
	case *time.Duration:
		nargv, arg, err := optArg(argv)
		if err != nil {
			return nil, fmt.Errorf("option '%s': %s", d.name, err)
		}
		argv = nargv
		*vp, err = time.ParseDuration(arg)
		if err != nil {
			return nil, fmt.Errorf("option '%s': wrong value '%s'", d.name, arg)
		}
	case *time.Time:
		nargv, arg, err := optArg(argv)
		if err != nil {
			return nil, fmt.Errorf("option '%s': %s", d.name, err)
		}
		argv = nargv
		*vp, err = ParseTime(arg)
		if err != nil {
			return nil, fmt.Errorf("option '%s': %s", d.name)
		}
	default:
		return nil, fmt.Errorf("unknown option type '%s'", d.name)
	}
	return argv, nil
}

func ParseTime(arg string) (time.Time, error) {
	var t time.Time
	for i := 0; i < len(tfmts); i++ {
		t, err := time.Parse(tfmts[i], arg)
		if err == nil {
			return t, nil
		}
	}
	return t, fmt.Errorf("wrong time format '%s'", arg)
}
