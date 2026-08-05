package filter

import "strings"

// MapToolValues applies fn to every string of a tool argument or result map,
// descending into nested maps and slices. Tool payloads are converted from and
// to JSON by the tool implementation, so only the container types that this
// conversion produces are walked.
//
// The payload is modified in place instead of being replaced by a copy: a
// plugin callback that returns a new map ends the callback chain of the
// runner, so the filters running after it would never see the payload.
func MapToolValues(m map[string]any, fn func(string) string) {
	for key, value := range m {
		m[key] = mapValue(value, fn)
	}
}

// mapValue applies fn to every string contained in value.
func mapValue(value any, fn func(string) string) any {
	switch v := value.(type) {
	case string:
		return fn(v)
	case map[string]any:
		MapToolValues(v, fn)
	case []any:
		for i, elem := range v {
			v[i] = mapValue(elem, fn)
		}
	case map[string]string:
		for key, s := range v {
			v[key] = fn(s)
		}
	case []string:
		for i, s := range v {
			v[i] = fn(s)
		}
	}
	return value
}

// ToolValuesText joins all strings of a tool payload so that generated
// replacements can be checked against the payload they are inserted into, the
// same way the full input text is used for the model request.
func ToolValuesText(m map[string]any) string {
	var sb strings.Builder
	MapToolValues(m, func(s string) string {
		sb.WriteString(s)
		sb.WriteString(" ")
		return s
	})
	return sb.String()
}
