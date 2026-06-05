// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package cmdctx

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

// GetString returns FlagValues[key] as a string, converting bool/int/float64 if needed.
func GetString(fv map[string]any, key string) string {
	switch v := fv[key].(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

// GetBool returns FlagValues[key] as a bool, parsing strings like "true"/"1" if needed.
func GetBool(fv map[string]any, key string) bool {
	switch v := fv[key].(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	case float64:
		return v != 0
	}
	return false
}

// GetStringSlice returns FlagValues[key] as []string, or nil on miss or wrong type.
func GetStringSlice(fv map[string]any, key string) []string {
	v, _ := fv[key].([]string)
	return v
}

// GetInt returns FlagValues[key] as an int, parsing strings if needed (hex/octal ok).
func GetInt(fv map[string]any, key string) int {
	switch v := fv[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case bool:
		if v {
			return 1
		}
		return 0
	case string:
		n, _ := strconv.ParseInt(v, 0, 64)
		return int(n)
	}
	return 0
}

// GetFloat64 returns FlagValues[key] as a float64, parsing strings if needed.
func GetFloat64(fv map[string]any, key string) float64 {
	switch v := fv[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case bool:
		if v {
			return 1
		}
		return 0
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

// Exists reports whether key is present in fv (value may be a zero value).
func Exists(fv map[string]any, key string) bool {
	_, ok := fv[key]
	return ok
}

// SlurpInputFile reads the file path stored at FlagValues["file"].
// "-" reads from stdin; "/dev/fd/N" works for process substitution.
// Must be called at endpoint execution time — not at ctx-construction time —
// so that the plugin exec path inherits stdin rather than consuming it early.
func SlurpInputFile(fv map[string]any) (string, error) {
	filePath := GetString(fv, "file")
	var r io.Reader
	switch filePath {
	case "":
		return "", fmt.Errorf("no input: use -f <file> or -f - to read from stdin")
	case "-":
		r = os.Stdin
	default:
		f, err := os.Open(filePath)
		if err != nil {
			return "", fmt.Errorf("opening -f: %w", err)
		}
		defer f.Close()
		r = f
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return string(b), nil
}
