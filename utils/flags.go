package utils

import (
	"fmt"
	"strconv"
	"strings"
)

const DisableUsernamePluginFlag = "disable-username-plugin"

// SplitOwnFlags reports whether DisableUsernamePluginFlag was given and returns
// the arguments with it removed. Everything else, -h and the sublauncher
// keywords included, is passed through untouched.
func SplitOwnFlags(args []string) (bool, []string, error) {
	disabled := false
	rest := make([]string, 0, len(args))

	for i, arg := range args {
		// A bare "--" ends flag parsing, the launcher gets the remainder as is.
		if arg == "--" {
			rest = append(rest, args[i:]...)
			break
		}

		name, isFlag := strings.CutPrefix(arg, "--")
		if !isFlag {
			name, isFlag = strings.CutPrefix(arg, "-")
		}
		name, value, hasValue := strings.Cut(name, "=")
		if !isFlag || name != DisableUsernamePluginFlag {
			rest = append(rest, arg)
			continue
		}

		// As in the flag package, a boolean is only settable as -flag=value.
		if !hasValue {
			disabled = true
			continue
		}
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return false, nil, fmt.Errorf("invalid boolean value %q for -%s", value, DisableUsernamePluginFlag)
		}
		disabled = parsed
	}

	return disabled, rest, nil
}
