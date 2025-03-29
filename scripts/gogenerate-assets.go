//go:build ignore
// +build ignore

package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Clean and create dist directories
	os.RemoveAll("assets/dist")
	createDirs := []string{"html", "js"}
	for _, dir := range createDirs {
		if err := os.MkdirAll(filepath.Join("assets", "dist", dir), 0755); err != nil {
			log.Fatal(err)
		}
	}

	// 1. Process JavaScript
	if err := processJS(); err != nil {
		log.Fatalf("JS processing failed: %v", err)
	}

	// 2. Copy HTML files
	//if err := copyHTML(); err != nil {
	//	log.Fatalf("HTML copy failed: %v", err)
	//}

	// 3. Gzip all assets
	if err := gzipAssets(); err != nil {
		log.Fatalf("Gzip failed: %v", err)
	}
}

		//"--bundle",
func processJS() error {
	cmd := exec.Command("./esbuild",
		"assets/src/js/restinpieces.js",
		"--bundle",
		//"--bundle=./assets/src/js/main.js",
		"--minify",
		"--drop:console",
		"--format=esm",
		"--target=es2017",
		"--platform=browser",
		"--outfile=assets/dist/js/restinpieces.min.js",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func copyHTML() error {
	return filepath.Walk("assets/src/html", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		relPath, _ := filepath.Rel("assets/src/html", path)
		dest := filepath.Join("assets/dist/html", relPath)

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
	return filepath.Walk("assets/dist", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		cmd := exec.Command("gzip", "-kf", path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
}
