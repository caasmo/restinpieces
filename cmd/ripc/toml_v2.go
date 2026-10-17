package main

import (
	"fmt"
	"os"

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
func tomlValueFromFile(filePath string) (string, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to read from path '%s': %w", ErrReadFile, filePath, err)
	}
	return string(fileContent), nil
}
