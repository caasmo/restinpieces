package dbtest

import (
	"bytes"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

type ConfigSuite struct {
	Db db.DbConfig
}

func (s ConfigSuite) TestGetAndInsertConfig(t *testing.T) {
	db := s.Db

	// 1. Verify table is initially empty
	content, format, err := db.GetConfig("app", 0)
	if err != nil {
		t.Fatalf("GetConfig from empty table failed: %v", err)
	}
	if content != nil {
		t.Errorf("expected nil content from empty table, got %s", content)
	}
	if format != "" {
		t.Errorf("expected empty format from empty table, got %s", format)
	}

	// 2. Insert configurations
	tests := []struct {
		scope       string
		content     []byte
		format      string
		description string
	}{
		{"app", []byte("v1"), "toml", "first version"},
		{"other", []byte("vA"), "json", "other scope"},
		{"app", []byte("v2"), "toml", "second version"},
	}

	for _, tt := range tests {
		err := db.InsertConfig(tt.scope, tt.content, tt.format, tt.description)
		if err != nil {
			t.Fatalf("InsertConfig failed: %v", err)
		}
	}

	// 3. Test GetConfig
	t.Run("GetLatestAppConfig", func(t *testing.T) {
		content, format, err := db.GetConfig("app", 0)
		if err != nil {
			t.Fatalf("GetConfig failed: %v", err)
		}
		if format != "toml" {
			t.Errorf("expected format 'toml', got '%s'", format)
		}
		if !bytes.Equal(content, []byte("v2")) {
			t.Errorf("expected content 'v2', got '%s'", content)
		}
	})

	t.Run("GetPreviousAppConfig", func(t *testing.T) {
		content, format, err := db.GetConfig("app", 1)
		if err != nil {
			t.Fatalf("GetConfig failed: %v", err)
		}
		if format != "toml" {
			t.Errorf("expected format 'toml', got '%s'", format)
		}
		if !bytes.Equal(content, []byte("v1")) {
			t.Errorf("expected content 'v1', got '%s'", content)
		}
	})

	t.Run("GetOtherScopeConfig", func(t *testing.T) {
		content, format, err := db.GetConfig("other", 0)
		if err != nil {
			t.Fatalf("GetConfig failed: %v", err)
		}
		if format != "json" {
			t.Errorf("expected format 'json', got '%s'", format)
		}
		if !bytes.Equal(content, []byte("vA")) {
			t.Errorf("expected content 'vA', got '%s'", content)
		}
	})

	// 4. Test edge cases
	t.Run("NonExistentScope", func(t *testing.T) {
		content, _, err := db.GetConfig("nonexistent", 0)
		if err != nil {
			t.Fatalf("GetConfig failed for nonexistent scope: %v", err)
		}
		if content != nil {
			t.Errorf("expected nil content for nonexistent scope, got %s", content)
		}
	})

	t.Run("GenerationOutOfBounds", func(t *testing.T) {
		content, _, err := db.GetConfig("app", 2)
		if err != nil {
			t.Fatalf("GetConfig failed for out-of-bounds generation: %v", err)
		}
		if content != nil {
			t.Errorf("expected nil content for out-of-bounds generation, got %s", content)
		}
	})
}

func (s ConfigSuite) RunAll(t *testing.T) {
	s.TestGetAndInsertConfig(t)
}
