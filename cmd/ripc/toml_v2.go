package main

import (
	"fmt"
	"os"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// tomlValueFromText returns the value the text names. The text is read as a
// TOML value, and text that is not valid TOML is used as a plain string.
func tomlValueFromText(text string) (interface{}, error) {
	var parsed map[string]interface{}
	err := toml.Unmarshal([]byte("temp_key = "+text), &parsed)
	if err != nil {
		err = toml.Unmarshal([]byte(fmt.Sprintf("temp_key = %q", text)), &parsed)
		if err != nil {
			return nil, fmt.Errorf("%w: could not parse '%s': %w", ErrParseValue, text, err)
		}
	}
	return parsed["temp_key"], nil
}

// tomlValueFromFile returns the contents of the named file as a string value.
// A text file ends with one line break that is not part of the value, so it is
// dropped; get writes that same break when it exports a value, so the pair
// round-trips, a value that itself ends with a line break included.
func tomlValueFromFile(filePath string) (string, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to read from path '%s': %w", ErrReadFile, filePath, err)
	}

	content := string(fileContent)
	if strings.HasSuffix(content, "\n") {
		content = strings.TrimSuffix(content, "\n")
		content = strings.TrimSuffix(content, "\r")
	}
	return content, nil
}

// isFileArgument reports whether the raw command value names a file to read,
// an argument that starts with "@".
func isFileArgument(rawValue string) bool {
	return strings.HasPrefix(rawValue, "@")
}
