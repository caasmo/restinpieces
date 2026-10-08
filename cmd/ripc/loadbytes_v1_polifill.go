// loadbytes_v1_polifill.go keeps the comments in the configuration when ripc
// changes a value.
//
// The commands that read and edit the config work with go-toml v1, and v1
// cannot read comments. A command like set therefore loads a config without
// its comments and saves it back without them: after one change, every
// explanation above a setting is gone. go-toml v2 added a parser that can read
// comments (the unstable package), but the editor that would change a
// document and write it back has not been developed yet.
//
// This file adds that missing pair:
//
//   - TomlParser parses a TOML document and returns the *toml.Tree type ripc
//     already uses, with the comments kept on their keys and tables, plus the
//     matched leaves with their comments and values. It replaces
//     toml.LoadBytes.
//   - SetWithComment changes the value at a path in that tree and keeps the
//     comment written above that value.
package main

import (
	"fmt"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml"
	"github.com/pelletier/go-toml/v2/unstable"
)

// TomlEntry is one filtered leaf: the value shown to the user and the comment
// written above it.
type TomlEntry struct {
	Comment string
	Value   interface{}
}

// TomlParser parses one TOML document. NewTomlParser builds it with the
// document and an empty tree, and Parse fills that tree and the entries.
type TomlParser struct {
	tomlBytes []byte
	filter    string
	tree      *toml.Tree
	entries   map[string]TomlEntry
}

// NewTomlParser returns a parser for the one TOML document in tomlBytes, with
// an empty tree and entries ready for Parse.
func NewTomlParser(tomlBytes []byte, filter string) (*TomlParser, error) {
	tree, err := toml.TreeFromMap(map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	return &TomlParser{
		tomlBytes: tomlBytes,
		filter:    filter,
		tree:      tree,
		entries:   make(map[string]TomlEntry),
	}, nil
}

// Parse parses the document with the v2 unstable parser and returns a v1 tree
// that carries the comments written above keys and table headers, plus the
// matched leaves in the entries map. It is the comment-preserving stand-in
// for toml.LoadBytes.
func (p *TomlParser) Parse() (*toml.Tree, map[string]TomlEntry, error) {
	var parser unstable.Parser
	parser.KeepComments = true
	parser.Reset(p.tomlBytes)

	var tableKey string
	var pendingComment string
	for parser.NextExpression() {
		expr := parser.Expression()

		switch expr.Kind {
		case unstable.Comment:
			pendingComment = commentLine(pendingComment, expr)
		case unstable.Table:
			tableKey = dottedKeyOf(expr)
			err := p.setTableComment(tableKey, pendingComment)
			if err != nil {
				return nil, nil, err
			}
			pendingComment = ""
		case unstable.KeyValue:
			dottedKey := joinDottedKey(tableKey, dottedKeyOf(expr))
			value := expr.Value()
			opts := toml.SetOptions{Comment: pendingComment}
			if value.Kind == unstable.String {
				raw := string(parser.Raw(value.Raw))
				opts.Multiline = strings.HasPrefix(raw, `"""`) || strings.HasPrefix(raw, `'''`)
				opts.Literal = strings.HasPrefix(raw, `'`)
			}
			err := p.setKeyValue(dottedKey, value, opts)
			if err != nil {
				return nil, nil, err
			}
			pendingComment = ""
		case unstable.ArrayTable:
			return nil, nil, fmt.Errorf("array table '%s' is not supported", dottedKeyOf(expr))
		}
	}

	err := parser.Error()
	if err != nil {
		return nil, nil, err
	}
	return p.tree, p.entries, nil
}

// SetWithComment changes the value at path and keeps the comment that sits on
// it. The trick is to reach the value through the tree's Values() map: the
// wrapper found there is a *toml.PubTOMLValue, which already carries the
// comment, so updating the value on that wrapper leaves the comment in place.
// The plain tree.Set replaces the wrapper and drops the comment, so it is only
// used when the path cannot be reached through Values().
func SetWithComment(tree *toml.Tree, path string, value interface{}) {
	keys := strings.Split(path, ".")
	node := tree
	for _, key := range keys[:len(keys)-1] {
		sub, ok := node.Values()[key].(*toml.Tree)
		if !ok {
			tree.Set(path, value)
			return
		}
		node = sub
	}

	leaf, ok := node.Values()[keys[len(keys)-1]].(*toml.PubTOMLValue)
	if !ok {
		tree.Set(path, value)
		return
	}
	leaf.SetValue(value)
}

// setKeyValue stores value in the parser's tree under dottedKey, with the
// comment and string style written above the key. An inline table is flattened
// so each of its keys becomes its own dotted key. Matched leaves are recorded
// in entries with their comment and value.
func (p *TomlParser) setKeyValue(dottedKey string, value *unstable.Node, opts toml.SetOptions) error {
	if value.Kind == unstable.InlineTable {
		for child := value.Children(); child.Next(); {
			if child.Node().Kind == unstable.Comment {
				continue
			}
			keyValue := child.Node()
			childKey := joinDottedKey(dottedKey, dottedKeyOf(keyValue))
			err := p.setKeyValue(childKey, keyValue.Value(), toml.SetOptions{})
			if err != nil {
				return err
			}
		}
		return nil
	}

	goValue, err := tomlValueOf(value)
	if err != nil {
		return err
	}
	if strings.Contains(dottedKey, p.filter) {
		p.entries[dottedKey] = TomlEntry{Comment: opts.Comment, Value: goValue}
	}
	p.tree.SetWithOptions(dottedKey, opts, goValue)
	return nil
}

// commentLine appends one comment line to the pending comment, without the
// leading "#".
func commentLine(pending string, node *unstable.Node) string {
	line := strings.TrimSpace(strings.TrimPrefix(string(node.Data), "#"))
	if pending == "" {
		return line
	}
	return pending + "\n" + line
}

// setTableComment attaches comment to the table named by tableKey, creating
// the table when the document has not created it yet. A table with no keys is
// created as well: the document declares it, so the tree must answer for it.
func (p *TomlParser) setTableComment(tableKey string, comment string) error {
	value := p.tree.Get(tableKey)
	if value == nil {
		sub, err := toml.TreeFromMap(map[string]interface{}{})
		if err != nil {
			return err
		}
		p.tree.SetWithComment(tableKey, comment, false, sub)
		return nil
	}

	sub, ok := value.(*toml.Tree)
	if !ok {
		return fmt.Errorf("table '%s' is already a value", tableKey)
	}
	if comment != "" {
		sub.SetComment(comment)
	}
	return nil
}

// tomlValueOf converts a value node into the Go value the v1 parser returned
// for the same input: string, bool, int64, float64, or a slice.
func tomlValueOf(node *unstable.Node) (interface{}, error) {
	switch node.Kind {
	case unstable.String:
		return string(node.Data), nil

	case unstable.Bool:
		return string(node.Data) == "true", nil

	case unstable.Integer:
		return strconv.ParseInt(strings.ReplaceAll(string(node.Data), "_", ""), 0, 64)

	case unstable.Float:
		return strconv.ParseFloat(strings.ReplaceAll(string(node.Data), "_", ""), 64)

	case unstable.Array:
		items := make([]interface{}, 0)
		for child := node.Children(); child.Next(); {
			if child.Node().Kind == unstable.Comment {
				continue
			}
			item, err := tomlValueOf(child.Node())
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil

	case unstable.InlineTable:
		table := make(map[string]interface{})
		for child := node.Children(); child.Next(); {
			if child.Node().Kind == unstable.Comment {
				continue
			}
			keyValue := child.Node()
			value, err := tomlValueOf(keyValue.Value())
			if err != nil {
				return nil, err
			}
			table[dottedKeyOf(keyValue)] = value
		}
		return table, nil

	case unstable.LocalDate, unstable.LocalTime, unstable.LocalDateTime, unstable.DateTime:
		return string(node.Data), nil
	}

	return nil, fmt.Errorf("unsupported TOML value kind %s", node.Kind)
}

// dottedKeyOf returns the dotted key of a table, array table, or key-value
// node.
func dottedKeyOf(node *unstable.Node) string {
	var parts []string
	for part := node.Key(); part.Next(); {
		parts = append(parts, string(part.Node().Data))
	}
	return strings.Join(parts, ".")
}

// joinDottedKey joins a table's dotted key and a key into one dotted key.
func joinDottedKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}
