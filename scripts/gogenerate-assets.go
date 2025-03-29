//go:build ignore
// +build ignore

package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/evanw/esbuild/pkg/api"
)

func main() {
	// Clean and create dist directories
	os.RemoveAll("public/dist")
	createDirs := []string{"html", "js"}
	for _, dir := range createDirs {
		if err := os.MkdirAll(filepath.Join("public", "dist", dir), 0755); err != nil {
			log.Fatal(err)
		}
	}

	// 1. Process JavaScript
	if err := processJS(); err != nil {
		log.Fatalf("JS processing failed: %v", err)
	}

	// 2. Copy HTML files
	if err := copyHTML(); err != nil {
		log.Fatalf("HTML copy failed: %v", err)
	}

	// 3. Gzip all assets
	if err := gzipAssets(); err != nil {
		log.Fatalf("Gzip failed: %v", err)
	}
}

func processJS() error {
	result := api.Build(api.BuildOptions{
		EntryPoints:       []string{"public/src/js/restinpieces.js"},
		Bundle:            true,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Drop:              api.DropConsole,
		Format:            api.FormatESModule,
		Target:            api.ES2017,
		Platform:          api.PlatformBrowser,
		Outfile:           "public/dist/js/restinpieces.min.js",
		Write:             true,
	})

	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			log.Printf("ESBuild error: %s", err.Text)
		}
		return fmt.Errorf("ESBuild failed with %d errors", len(result.Errors))
	}
	return nil
}

func copyHTML() error {
	return filepath.Walk("public/src/html", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		relPath, _ := filepath.Rel("public/src/html", path)
		dest := filepath.Join("public/dist/html", relPath)

		// Create destination directory
		os.MkdirAll(filepath.Dir(dest), 0755)

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		destFile, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		return err
	})
}

func gzipAssets() error {
	return filepath.Walk("public/dist", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		// Open the original file
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		// Create output gzip file
		out, err := os.Create(path + ".gz")
		if err != nil {
			return err
		}
		defer out.Close()

		// Create gzip writer
		gz := gzip.NewWriter(out)
		defer gz.Close()

		// Copy content
		_, err = io.Copy(gz, in)
		return err
	})
}
