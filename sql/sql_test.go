package sql

import (
	"io/fs"
	"reflect"
	"sort"
	"testing"
)

func TestSchemaAccess(t *testing.T) {
	expectedFiles := []string{
		"app/app_config.sql",
		"app/job_queue.sql",
		"app/users.sql",
		"log/logs.sql",
	}

	var foundFiles []string
	sqlFS := FS()

	err := fs.WalkDir(sqlFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			foundFiles = append(foundFiles, path)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk embedded sql files: %v", err)
	}

	sort.Strings(expectedFiles)
	sort.Strings(foundFiles)

	if !reflect.DeepEqual(expectedFiles, foundFiles) {
		t.Errorf("mismatch in embedded sql files.\nGot:  %v\nWant: %v", foundFiles, expectedFiles)
	}
}
