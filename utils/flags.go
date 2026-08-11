// Package utils provides common utility functions and tools, including command-line flag parsing.
package utils

import (
	"os"
	"strings"
)

// BoolFlag represents a boolean command-line flag (e.g., --disable-username-plugin).
type BoolFlag struct {
	// Name is the long flag name (without dashes).
	Name string
	// Bool holds the current value of the flag.
	Bool bool
	// Aliases are alternative names for the flag.
	Aliases []string
	// defaultVal stores the default value when the flag is not specified.
	defaultVal bool
}

// StringFlag represents a string-valued command-line flag (e.g., --prompt).
type StringFlag struct {
	// Name is the long flag name (without dashes).
	Name string
	// Value holds the current value of the flag.
	Value string
	// Aliases are alternative names for the flag.
	Aliases []string
	// defaultVal stores the default value when the flag is not specified.
	defaultVal string
}

// FlagSet manages a collection of boolean and string flags for parsing command-line arguments.
type FlagSet struct {
	// boolFlags holds all boolean flags in this set.
	boolFlags []*BoolFlag
	// stringFlags holds all string flags in this set.
	stringFlags []*StringFlag
	// params holds parsed split parameters (currently unused).
	params *SplitParams
}

// SplitParams holds the remaining arguments after flag parsing.
type SplitParams struct {
	// Args contains the non-flag command-line arguments.
	Args []string
}

// NewFlagSet creates a new FlagSet with the provided boolean and string flags.
// boolFlags can be a mix of *BoolFlag and *StringFlag instances.
func NewFlagSet(boolFlags ...interface{}) *FlagSet {
	fs := &FlagSet{}
	for _, f := range boolFlags {
		switch flag := f.(type) {
		case *BoolFlag:
			fs.boolFlags = append(fs.boolFlags, flag)
		case *StringFlag:
			fs.stringFlags = append(fs.stringFlags, flag)
		}
	}
	return fs
}

// SplitOwnFlags parses command-line arguments and separates flags from positional arguments.
// It updates flag values based on the provided args and returns:
//   - hasBoolFlags: true if any boolean flag was set
//   - prompt: the value of the first string flag (typically "prompt")
//   - newArgs: the remaining non-flag arguments
//   - error: nil (currently always returns nil)
func (fs *FlagSet) SplitOwnFlags(args []string) (hasBoolFlags bool, prompt string, newArgs []string, error error) {
	boolFlagMap := make(map[string]*BoolFlag)
	for _, f := range fs.boolFlags {
		boolFlagMap[f.Name] = f
		for _, alias := range f.Aliases {
			boolFlagMap[alias] = f
		}
		f.Bool = f.defaultVal
	}

	stringFlagMap := make(map[string]*StringFlag)
	for _, f := range fs.stringFlags {
		stringFlagMap[f.Name] = f
		for _, alias := range f.Aliases {
			stringFlagMap[alias] = f
		}
		f.Value = f.defaultVal
	}

	var newArgList []string
	i := 0
	for i < len(args) {
		arg := args[i]

		if strings.HasPrefix(arg, "-") {
			if strings.HasPrefix(arg, "--") {
				parts := strings.SplitN(arg[2:], "=", 2)
				flagName := parts[0]

				if f, exists := boolFlagMap[flagName]; exists {
					f.Bool = true
					i++
					continue
				}

				if f, exists := stringFlagMap[flagName]; exists {
					if len(parts) == 2 {
						f.Value = parts[1]
					} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
						f.Value = args[i+1]
						i++
					}
					i++
					continue
				}
			} else {
				parts := strings.SplitN(arg[1:], "=", 2)
				flagName := parts[0]

				if f, exists := boolFlagMap[flagName]; exists {
					f.Bool = true
					i++
					continue
				}

				if f, exists := stringFlagMap[flagName]; exists {
					if len(parts) == 2 {
						f.Value = parts[1]
					} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
						f.Value = args[i+1]
						i++
					}
					i++
					continue
				}
			}
		}

		newArgList = append(newArgList, arg)
		i++
	}

	os.Args = append(os.Args[:1], newArgList...)

	hasAny := false
	for _, f := range fs.boolFlags {
		if f.Bool {
			hasAny = true
			break
		}
	}

	var promptValue string
	if len(fs.stringFlags) > 0 {
		promptValue = fs.stringFlags[0].Value
	}

	return hasAny, promptValue, newArgList, nil
}

// NewBoolFlag creates a new boolean flag with the given name and default value.
func NewBoolFlag(name string, defaultValue bool) *BoolFlag {
	return &BoolFlag{
		Name:       name,
		defaultVal: defaultValue,
	}
}

// NewStringFlag creates a new string flag with the given name and default value.
func NewStringFlag(name string, defaultValue string) *StringFlag {
	return &StringFlag{
		Name:       name,
		defaultVal: defaultValue,
	}
}

// SetAliases adds alternative names for the flag that can be used interchangeably.
func (f *BoolFlag) SetAliases(aliases ...string) *BoolFlag {
	f.Aliases = aliases
	return f
}

// SetAliases adds alternative names for the flag that can be used interchangeably.
func (f *StringFlag) SetAliases(aliases ...string) *StringFlag {
	f.Aliases = aliases
	return f
}

// SplitOwnFlags is a convenience function that parses command-line arguments using predefined flags.
// It returns the same values as FlagSet.SplitOwnFlags with predefined flags for "disable-username-plugin" and "prompt".
func SplitOwnFlags(args []string) (bool, string, []string, error) {
	flagSet := NewFlagSet(
		NewBoolFlag("disable-username-plugin", false).SetAliases("disable_username_plugin"),
		NewStringFlag("prompt", "").SetAliases("prompt_text", "p"),
	)

	return flagSet.SplitOwnFlags(args)
}
