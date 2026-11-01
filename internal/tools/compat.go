// Derived from grafana/cog PR #1098. Modified for cog-sharp; see NOTICE.

package tools

import (
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var nonAlphaNumRegex = regexp.MustCompile("[^a-zA-Z0-9 ]+")

func UpperCamelCase(value string) string {
	value = LowerCamelCase(value)
	if len(value) > 0 {
		value = strings.ToUpper(value[:1]) + value[1:]
	}
	return value
}

func LowerCamelCase(value string) string {
	value = nonAlphaNumRegex.ReplaceAllString(value, " ")
	value = cases.Title(language.AmericanEnglish, cases.NoLower).String(value)
	value = strings.ReplaceAll(value, " ", "")
	if len(value) > 0 {
		value = strings.ToLower(value[:1]) + value[1:]
	}
	return value
}

func Map[T any, O any](input []T, mapper func(T) O) []O {
	if input == nil {
		return nil
	}
	output := make([]O, len(input))
	for i := range input {
		output[i] = mapper(input[i])
	}
	return output
}

func AnyToInt64(value any) int64 {
	switch value := value.(type) {
	case int:
		return int64(value)
	case int8:
		return int64(value)
	case int16:
		return int64(value)
	case int32:
		return int64(value)
	case uint64:
		return int64(value)
	case int64:
		return value
	case float32:
		return int64(value)
	case float64:
		return int64(value)
	}
	return value.(int64)
}
