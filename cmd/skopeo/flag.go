package main

import (
	"strconv"

	"github.com/spf13/pflag"
)

// optionalBool is a boolean with a separate presence flag.
type optionalBool struct {
	present bool
	value   bool
}

// optionalBool is a cli.Generic == flag.Value implementation equivalent to
// the one underlying flag.Bool, except that it records whether the flag has been set.
// This is distinct from optionalBool to (pretend to) force callers to use
// optionalBoolFlag
type optionalBoolValue optionalBool

func optionalBoolFlag(fs *pflag.FlagSet, p *optionalBool, name, usage string) *pflag.Flag {
	flag := fs.VarPF(internalNewOptionalBoolValue(p), name, "", usage)
	flag.NoOptDefVal = "true"
	return flag
}

// WARNING: Do not directly use this method to define optionalBool flag.
// Caller should use optionalBoolFlag
func internalNewOptionalBoolValue(p *optionalBool) pflag.Value {
	p.present = false
	return (*optionalBoolValue)(p)
}

func (ob *optionalBoolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	ob.value = v
	ob.present = true
	return nil
}

func (ob *optionalBoolValue) String() string {
	if !ob.present {
		return "" // This is, sadly, not round-trip safe: --flag is interpreted as --flag=true
	}
	return strconv.FormatBool(ob.value)
}

func (ob *optionalBoolValue) Type() string {
	return "bool"
}

func (ob *optionalBoolValue) IsBoolFlag() bool {
	return true
}

// optionalString is a string with a separate presence flag.
type optionalString struct {
	present bool
	value   string
}

// optionalString is a cli.Generic == flag.Value implementation equivalent to
// the one underlying flag.String, except that it records whether the flag has been set.
// This is distinct from optionalString to (pretend to) force callers to use
// newoptionalString
type optionalStringValue optionalString

func newOptionalStringValue(p *optionalString) pflag.Value {
	p.present = false
	return (*optionalStringValue)(p)
}

func (ob *optionalStringValue) Set(s string) error {
	ob.value = s
	ob.present = true
	return nil
}

func (ob *optionalStringValue) String() string {
	if !ob.present {
		return "" // This is, sadly, not round-trip safe: --flag= is interpreted as {present:true, value:""}
	}
	return ob.value
}

func (ob *optionalStringValue) Type() string {
	return "string"
}

// optionalInt is a int with a separate presence flag.
type optionalInt struct {
	present bool
	value   int
}

// optionalInt is a cli.Generic == flag.Value implementation equivalent to
// the one underlying flag.Int, except that it records whether the flag has been set.
// This is distinct from optionalInt to (pretend to) force callers to use
// newoptionalIntValue
type optionalIntValue optionalInt

func newOptionalIntValue(p *optionalInt) pflag.Value {
	p.present = false
	return (*optionalIntValue)(p)
}

func (ob *optionalIntValue) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, strconv.IntSize)
	if err != nil {
		return err
	}
	ob.value = int(v)
	ob.present = true
	return nil
}

func (ob *optionalIntValue) String() string {
	if !ob.present {
		return "" // If the value is not present, just return an empty string, any other value wouldn't make sense.
	}
	return strconv.Itoa(int(ob.value))
}

func (ob *optionalIntValue) Type() string {
	return "int"
}
