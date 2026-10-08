package main

import (
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml"
)

// commentAt returns the comment attached to the leaf or table at path.
func commentAt(t *testing.T, tree *toml.Tree, path string) string {
	t.Helper()

	keys := strings.Split(path, ".")
	node := tree.Values()
	for _, key := range keys[:len(keys)-1] {
		sub, ok := node[key].(*toml.Tree)
		if !ok {
			t.Fatalf("commentAt(%q): no table at '%s'", path, key)
		}
		node = sub.Values()
	}

	switch entry := node[keys[len(keys)-1]].(type) {
	case *toml.PubTOMLValue:
		return entry.Comment()
	case *toml.Tree:
		return entry.Comment()
	}
	t.Fatalf("commentAt(%q): no entry", path)
	return ""
}

// parseDoc parses doc with no filter and returns the tree.
func parseDoc(t *testing.T, doc string) *toml.Tree {
	t.Helper()

	tree, _ := parseFiltered(t, doc, "")
	return tree
}

// parseFiltered parses doc with filter and returns the tree and matched entries.
func parseFiltered(t *testing.T, doc string, filter string) (*toml.Tree, map[string]TomlEntry) {
	t.Helper()

	parser, err := NewTomlParser([]byte(doc), filter)
	if err != nil {
		t.Fatalf("NewTomlParser failed: %v", err)
	}
	tree, entries, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return tree, entries
}

func TestTomlParser_Values(t *testing.T) {
	const doc = `title = "hello"
count = 42
ratio = 1.5
enabled = true
tags = ["a", "b"]
block_user_agent.agents = ["SemrushBot"]
oauth2 = { github = { name = "github", pkce = true } }

[server]
addr = ":8080"

[log.batch]
size = 200
`

	tree := parseDoc(t, doc)

	tests := []struct {
		path string
		want interface{}
	}{
		{path: "title", want: "hello"},
		{path: "count", want: int64(42)},
		{path: "ratio", want: 1.5},
		{path: "enabled", want: true},
		{path: "oauth2.github.name", want: "github"},
		{path: "oauth2.github.pkce", want: true},
		{path: "server.addr", want: ":8080"},
		{path: "log.batch.size", want: int64(200)},
	}

	for _, tt := range tests {
		got := tree.Get(tt.path)
		if got != tt.want {
			t.Errorf("tree.Get(%q) = %#v, want %#v", tt.path, got, tt.want)
		}
	}
}

func TestTomlParser_DateTimes(t *testing.T) {
	const doc = `d = 1979-05-27
t = 07:32:00
dt = 1979-05-27T07:32:00
instant = 1979-05-27T07:32:00Z
`

	tree := parseDoc(t, doc)

	tests := []struct {
		path string
		want string
	}{
		{path: "d", want: "1979-05-27"},
		{path: "t", want: "07:32:00"},
		{path: "dt", want: "1979-05-27T07:32:00"},
		{path: "instant", want: "1979-05-27T07:32:00Z"},
	}

	for _, tt := range tests {
		if got := tree.Get(tt.path); got != tt.want {
			t.Errorf("tree.Get(%q) = %v, want %q", tt.path, got, tt.want)
		}
	}
}

func TestTomlParser_Arrays(t *testing.T) {
	const doc = `tags = ["a", "b"]
block_user_agent.agents = ["SemrushBot"]
empty = []
mixed = [1, 2.5, "three", true]
nested = [[1, 2], [3]]
crates = [{name = "small"}, {name = "big"}]
commented = [1, # note
  2]
`

	tree := parseDoc(t, doc)

	if !equalItems(tree.Get("tags"), "a", "b") {
		t.Errorf("tags = %v, want [a b]", tree.Get("tags"))
	}
	if !equalItems(tree.Get("block_user_agent.agents"), "SemrushBot") {
		t.Errorf("block_user_agent.agents = %v, want [SemrushBot]", tree.Get("block_user_agent.agents"))
	}
	if !equalItems(tree.Get("empty")) {
		t.Errorf("empty = %v, want []", tree.Get("empty"))
	}
	if !equalItems(tree.Get("mixed"), int64(1), 2.5, "three", true) {
		t.Errorf("mixed = %v, want [1 2.5 three true]", tree.Get("mixed"))
	}
	if !equalItems(tree.Get("commented"), int64(1), int64(2)) {
		t.Errorf("commented = %v, want [1 2]", tree.Get("commented"))
	}

	nested, ok := tree.Get("nested").([]interface{})
	if !ok || len(nested) != 2 {
		t.Fatalf("nested = %v, want two items", tree.Get("nested"))
	}
	if !equalItems(nested[0], int64(1), int64(2)) {
		t.Errorf("nested[0] = %v, want [1 2]", nested[0])
	}
	if !equalItems(nested[1], int64(3)) {
		t.Errorf("nested[1] = %v, want [3]", nested[1])
	}

	crates, ok := tree.Get("crates").([]interface{})
	if !ok || len(crates) != 2 {
		t.Fatalf("crates = %v, want two items", tree.Get("crates"))
	}
	first, ok := crates[0].(map[string]interface{})
	if !ok || first["name"] != "small" {
		t.Errorf("crates[0] = %v, want map[name:small]", crates[0])
	}
	second, ok := crates[1].(map[string]interface{})
	if !ok || second["name"] != "big" {
		t.Errorf("crates[1] = %v, want map[name:big]", crates[1])
	}
}

// equalItems reports whether got is a slice holding exactly want, compared item
// by item because slices cannot be compared with ==.
func equalItems(got interface{}, want ...interface{}) bool {
	items, ok := got.([]interface{})
	if !ok || len(items) != len(want) {
		return false
	}
	for i := range items {
		if items[i] != want[i] {
			return false
		}
	}
	return true
}

func TestTomlParser_CommentsAboveKeys(t *testing.T) {
	const doc = `# Directory containing static web assets
public_dir = "/var/www"

[server]
# Address the server listens on
addr = ":8080"
# First line
# Second line
read_timeout = "5s"
`

	tree := parseDoc(t, doc)

	tests := []struct {
		path string
		want string
	}{
		{path: "public_dir", want: "Directory containing static web assets"},
		{path: "server.addr", want: "Address the server listens on"},
		{path: "server.read_timeout", want: "First line\nSecond line"},
	}

	for _, tt := range tests {
		if got := commentAt(t, tree, tt.path); got != tt.want {
			t.Errorf("comment at %q = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestTomlParser_CommentAboveTableHeader(t *testing.T) {
	const doc = `# HTTP server configuration
[server]
addr = ":8080"

# Batch settings
[log.batch]
size = 200
`

	tree := parseDoc(t, doc)

	if got := commentAt(t, tree, "server"); got != "HTTP server configuration" {
		t.Errorf("server comment = %q, want %q", got, "HTTP server configuration")
	}
	if got := commentAt(t, tree, "log.batch"); got != "Batch settings" {
		t.Errorf("log.batch comment = %q, want %q", got, "Batch settings")
	}
}

func TestTomlParser_CommentOnExistingTable(t *testing.T) {
	const doc = `[a.b]
x = 1

# Parent
[a]
y = 2
`

	tree := parseDoc(t, doc)

	if got := commentAt(t, tree, "a"); got != "Parent" {
		t.Errorf("a comment = %q, want %q", got, "Parent")
	}
	if got := tree.Get("a.y"); got != int64(2) {
		t.Errorf("a.y = %v, want 2", got)
	}
}

func TestTomlParser_EmptyTable(t *testing.T) {
	const doc = `[backup]
[oauth2]

[server]
  addr = ":8080"
`

	tree := parseDoc(t, doc)

	if !tree.Has("backup") {
		t.Error("a table with no keys vanished from the tree")
	}
	if !tree.Has("oauth2") {
		t.Error("an empty table vanished from the tree")
	}
	for _, key := range []string{"backup", "oauth2"} {
		sub, ok := tree.Get(key).(*toml.Tree)
		if !ok {
			t.Fatalf("%s is not a table", key)
		}
		if len(sub.Keys()) != 0 {
			t.Errorf("%s = %v, want no keys", key, sub.Keys())
		}
	}
	if got := tree.Get("server.addr"); got != ":8080" {
		t.Errorf("server.addr = %v, want :8080", got)
	}
}

func TestTomlParser_CommentRoundTrip(t *testing.T) {
	const doc = `# The public directory
public_dir = "/var/www"
`

	tree := parseDoc(t, doc)

	out, err := tree.ToTomlString()
	if err != nil {
		t.Fatalf("ToTomlString failed: %v", err)
	}

	want := "# The public directory\npublic_dir = \"/var/www\""
	if got := strings.TrimSpace(out); got != want {
		t.Errorf("marshaled tree = %q, want %q", got, want)
	}
}

func TestTomlParser_StringStyles(t *testing.T) {
	const doc = `pem = """
-----BEGIN-----
line
-----END-----
"""
raw = '''
C:\path
'''
`

	tree := parseDoc(t, doc)

	if got, want := tree.Get("pem"), "-----BEGIN-----\nline\n-----END-----\n"; got != want {
		t.Errorf("pem = %q, want %q", got, want)
	}
	if got, want := tree.Get("raw"), "C:\\path\n"; got != want {
		t.Errorf("raw = %q, want %q", got, want)
	}

	out, err := tree.ToTomlString()
	if err != nil {
		t.Fatalf("ToTomlString failed: %v", err)
	}
	if !strings.Contains(out, "pem = \"\"\"") {
		t.Errorf("pem lost its multiline form:\n%s", out)
	}
	if !strings.Contains(out, "raw = '''") {
		t.Errorf("raw lost its literal multiline form:\n%s", out)
	}
}

func TestTomlParser_SameLineCommentDropped(t *testing.T) {
	tree := parseDoc(t, "addr = \":8080\" # inline note\n")

	if got := commentAt(t, tree, "addr"); got != "" {
		t.Errorf("same-line comment kept: %q", got)
	}
}

func TestTomlParser_MalformedToml(t *testing.T) {
	parser, err := NewTomlParser([]byte("[server"), "")
	if err != nil {
		t.Fatalf("NewTomlParser failed: %v", err)
	}
	_, _, err = parser.Parse()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestTomlParser_ArrayTable(t *testing.T) {
	parser, err := NewTomlParser([]byte("[[jobs]]\nname = \"first\"\n"), "")
	if err != nil {
		t.Fatalf("NewTomlParser failed: %v", err)
	}
	_, _, err = parser.Parse()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("expected a 'not supported' error, got %v", err)
	}
}

func TestTomlParser_Empty(t *testing.T) {
	tree := parseDoc(t, "")
	if len(tree.Keys()) != 0 {
		t.Errorf("expected an empty tree, got keys %v", tree.Keys())
	}
}

func TestTomlParser_Filter(t *testing.T) {
	const doc = `oauth2 = { github = { name = "github" } }

[server]
# Port
port = 8080
addr = ":8080"

[log]
# Keep
size = 200
`

	cases := []struct {
		name    string
		filter  string
		want    []string
		missing []string
	}{
		{
			name:   "empty keeps all",
			filter: "",
			want:   []string{"server.port", "server.addr", "log.size", "oauth2.github.name"},
		},
		{
			name:    "substring match",
			filter:  "server",
			want:    []string{"server.port", "server.addr"},
			missing: []string{"log.size", "oauth2.github.name"},
		},
		{
			name:    "no match",
			filter:  "absent",
			missing: []string{"server.port", "log.size", "oauth2.github.name"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tree, entries := parseFiltered(t, doc, tt.filter)

			for _, path := range tt.want {
				if _, ok := entries[path]; !ok {
					t.Errorf("entries missing %q", path)
				}
			}
			for _, path := range tt.missing {
				if _, ok := entries[path]; ok {
					t.Errorf("entries should not have %q", path)
				}
				if tree.Get(path) == nil {
					t.Errorf("tree lost %q", path)
				}
			}
		})
	}

	tree, entries := parseFiltered(t, doc, "")

	port, ok := entries["server.port"]
	if !ok {
		t.Fatal("entries missing server.port")
	}
	if port.Value != int64(8080) {
		t.Errorf("server.port value = %v, want 8080", port.Value)
	}
	if port.Comment != "Port" {
		t.Errorf("server.port comment = %q, want %q", port.Comment, "Port")
	}
	if _, ok := entries["oauth2.github.name"]; !ok {
		t.Error("entries missing flattened inline leaf oauth2.github.name")
	}
	if got := tree.Get("log.size"); got != int64(200) {
		t.Errorf("tree lost log.size, got %v", got)
	}
}

func TestSetWithComment_KeepsComment(t *testing.T) {
	const doc = `# The public directory
public_dir = "/var/www"
`

	tree := parseDoc(t, doc)

	SetWithComment(tree, "public_dir", "/srv/www")

	if got := tree.Get("public_dir"); got != "/srv/www" {
		t.Errorf("public_dir = %v, want /srv/www", got)
	}
	if got := commentAt(t, tree, "public_dir"); got != "The public directory" {
		t.Errorf("comment = %q, want %q", got, "The public directory")
	}
}

func TestSetWithComment_PlainSetDropsComment(t *testing.T) {
	const doc = `# The public directory
public_dir = "/var/www"
`

	tree := parseDoc(t, doc)

	tree.Set("public_dir", "/srv/www")

	if got := commentAt(t, tree, "public_dir"); got != "" {
		t.Errorf("tree.Set kept the comment %q", got)
	}
}

func TestSetWithComment_NoComment(t *testing.T) {
	tree := parseDoc(t, "addr = \":8080\"\n")

	SetWithComment(tree, "addr", ":9090")

	if got := tree.Get("addr"); got != ":9090" {
		t.Errorf("addr = %v, want :9090", got)
	}
	if got := commentAt(t, tree, "addr"); got != "" {
		t.Errorf("unexpected comment %q", got)
	}
}

func TestSetWithComment_NewPath(t *testing.T) {
	tree := parseDoc(t, "[server]\naddr = \":8080\"\n")

	SetWithComment(tree, "server.port", int64(8080))

	if got := tree.Get("server.port"); got != int64(8080) {
		t.Errorf("server.port = %v, want 8080", got)
	}
	if got := commentAt(t, tree, "server.port"); got != "" {
		t.Errorf("unexpected comment %q", got)
	}
}

func TestSetWithComment_NewTable(t *testing.T) {
	tree := parseDoc(t, "addr = \":8080\"\n")

	SetWithComment(tree, "server.port", int64(8080))

	if got := tree.Get("server.port"); got != int64(8080) {
		t.Errorf("server.port = %v, want 8080", got)
	}
}

func TestSetWithComment_MultilineValue(t *testing.T) {
	const doc = `pem = """
-----BEGIN-----
-----END-----
"""
`

	tree := parseDoc(t, doc)

	SetWithComment(tree, "pem", "new\nvalue")

	out, err := tree.ToTomlString()
	if err != nil {
		t.Fatalf("ToTomlString failed: %v", err)
	}
	if !strings.Contains(out, "pem = \"\"\"") {
		t.Errorf("multiline form lost after set:\n%s", out)
	}
}
