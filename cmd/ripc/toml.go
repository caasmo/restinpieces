package main

import (
	toml "github.com/pelletier/go-toml"
)

// tomlTableOf returns the TOML table held by value and true when value is a
// table. It is the single place that tells tables apart from plain values.
func tomlTableOf(value interface{}) (*toml.Tree, bool) {
	table, ok := value.(*toml.Tree)
	return table, ok
}

// tomlArrayOf returns the items held by value and true when value is a TOML
// array. It is the single place that tells arrays apart from plain values.
func tomlArrayOf(value interface{}) ([]interface{}, bool) {
	array, ok := value.([]interface{})
	return array, ok
}
