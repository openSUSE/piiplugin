package utils

import (
	"os"
	"strings"
)

// BoolFlag describes a --disable-username-plugin style flag.
type BoolFlag struct {
	Name       string
	Bool       bool
	Aliases    []string
	defaultVal bool
}

// StringFlag describes a --prompt style flag.
type StringFlag struct {
	Name       string
	Value      string
	Aliases    []string
	defaultVal string
}

type FlagSet struct {
	boolFlags   []*BoolFlag
	stringFlags []*StringFlag
	params      *SplitParams
}

type SplitParams struct {
	Args []string
}

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

func NewBoolFlag(name string, defaultValue bool) *BoolFlag {
	return &BoolFlag{
		Name:       name,
		defaultVal: defaultValue,
	}
}

func NewStringFlag(name string, defaultValue string) *StringFlag {
	return &StringFlag{
		Name:       name,
		defaultVal: defaultValue,
	}
}

func (f *BoolFlag) SetAliases(aliases ...string) *BoolFlag {
	f.Aliases = aliases
	return f
}

func (f *StringFlag) SetAliases(aliases ...string) *StringFlag {
	f.Aliases = aliases
	return f
}

func SplitOwnFlags(args []string) (bool, string, []string, error) {
	flagSet := NewFlagSet(
		NewBoolFlag("disable-username-plugin", false).SetAliases("disable_username_plugin"),
		NewStringFlag("prompt", "").SetAliases("prompt_text", "p"),
	)

	return flagSet.SplitOwnFlags(args)
}
