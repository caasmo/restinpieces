//go:build ignore
// +build ignore

package main

import (
	"compress/gzip"
	"flag" // Import the flag package
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/evanw/esbuild/pkg/api"
)

func main() {
	// Define a command-line flag for the base directory
	baseDir := flag.String("baseDir", "public", "The base directory containing src and for dist output")
	flag.Parse() // Parse the command-line flags

	// Derive src and dist directories from the base directory
	srcDir := filepath.Join(*baseDir, "src")
	distDir := filepath.Join(*baseDir, "dist")

	log.Printf("Using Base Directory: %s", *baseDir)
	log.Printf("Source Directory: %s", srcDir)
	log.Printf("Distribution Directory: %s", distDir)

	// Clean and create dist directories
	log.Printf("Cleaning directory: %s", distDir)
	if err := os.RemoveAll(distDir); err != nil {
		// Ignore "does not exist" errors on removal, handle others
		if !os.IsNotExist(err) {
			log.Fatalf("Failed to clean dist directory %s: %v", distDir, err)
		}
	}

	createDirs := []string{"js", "css"} // Subdirectories to create within dist
	for _, dir := range createDirs {
		targetDir := filepath.Join(distDir, dir)
		log.Printf("Creating directory: %s", targetDir)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", targetDir, err)
		}
	}

	// 1. Process JavaScript
	log.Println("Processing JavaScript...")
	if err := processJS(srcDir, distDir); err != nil {
		log.Fatalf("JS processing failed: %v", err)
	}
	log.Println("JavaScript processing complete.")

	// 2. Copy HTML files
	log.Println("Copying HTML files...")
	if err := copyHTML(srcDir, distDir); err != nil {
		log.Fatalf("HTML copy failed: %v", err)
	}
	log.Println("HTML copy complete.")

	// 3. Gzip all assets
	log.Println("Gzipping assets...")
	if err := gzipAssets(distDir); err != nil {
		log.Fatalf("Gzip failed: %v", err)
	}
	log.Println("Gzipping complete.")
	log.Println("Build finished successfully.")
}

func processJS(srcDir, distDir string) error {
	entryPoint := filepath.Join(srcDir, "js", "restinpieces.js")
	outFile := filepath.Join(distDir, "js", "restinpieces.js")

	log.Printf("  Entry point: %s", entryPoint)
	log.Printf("  Output file: %s", outFile)

	result := api.Build(api.BuildOptions{
		EntryPoints:       []string{entryPoint},
		Bundle:            true,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Drop:              api.DropConsole,
		Format:            api.FormatESModule,
		Target:            api.ES2017,
		Platform:          api.PlatformBrowser,
		Outfile:           outFile,
		Write:             true,
	})

	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			log.Printf("  ESBuild error: %s (Location: %s:%d:%d)", err.Text, err.Location.File, err.Location.Line, err.Location.Column)
		}
		return fmt.Errorf("ESBuild failed with %d errors", len(result.Errors))
	}
	log.Printf("  Successfully processed %s", outFile)
	return nil
}

func copyHTML(srcDir, distDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("  Error accessing path %q: %v", path, err)
			return err // Propagate walk errors
		}
		if info.IsDir() {
			return nil // Skip directories
		}

		// Only process HTML files
		if filepath.Ext(path) != ".html" {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			log.Printf("  Error calculating relative path for %q from %q: %v", path, srcDir, err)
			return err // Should not happen if path is within srcDir
		}
		dest := filepath.Join(distDir, relPath)

		// Create destination directory if it doesn't exist
		destParentDir := filepath.Dir(dest)
		if err := os.MkdirAll(destParentDir, 0755); err != nil {
			log.Printf("  Error creating destination directory %q: %v", destParentDir, err)
			return err
		}

		// Copy file
		log.Printf("  Copying %s to %s", path, dest)
		srcFile, err := os.Open(path)
		if err != nil {
			log.Printf("  Error opening source file %q: %v", path, err)
			return err
		}
		defer srcFile.Close()

		destFile, err := os.Create(dest)
		if err != nil {
			log.Printf("  Error creating destination file %q: %v", dest, err)
			return err
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			log.Printf("  Error copying data from %q to %q: %v", path, dest, err)
		}
		return err // Return the result of io.Copy
	})
}

func gzipAssets(distDir string) error {
	return filepath.Walk(distDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("  Error accessing path %q: %v", path, err)
			return err
		}
		// Skip directories and already gzipped files
		if info.IsDir() || filepath.Ext(path) == ".gz" {
			return nil
		}

		// Define target gzip path
		gzipPath := path + ".gz"
		log.Printf("  Gzipping %s to %s", path, gzipPath)

		// Open the original file
		in, err := os.Open(path)
		if err != nil {
			log.Printf("  Error opening file %q for gzipping: %v", path, err)
			return err
		}
		defer in.Close()

		// Create output gzip file
		out, err := os.Create(gzipPath)
		if err != nil {
			log.Printf("  Error creating gzip file %q: %v", gzipPath, err)
			return err
		}
		defer out.Close()

		// Create gzip writer with default compression
		gz := gzip.NewWriter(out)
		// Consider using gzip.NewWriterLevel for different compression levels if needed
		// gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
		// if err != nil { ... }

		// Copy content
		_, err = io.Copy(gz, in)
		if err != nil {
			log.Printf("  Error gzipping data for %q: %v", path, err)
			// Clean up potentially partially written .gz file on error
			gz.Close() // Close writer first
			out.Close() // Close file handle
			os.Remove(gzipPath) // Attempt removal
			return err
		}

		// Important: Close the gzip writer to flush buffers and write footer
		err = gz.Close()
		if err != nil {
			log.Printf("  Error finalizing gzip stream for %q: %v", gzipPath, err)
			// Clean up potentially corrupt .gz file on close error
			out.Close() // Close file handle
			os.Remove(gzipPath) // Attempt removal
		}

		return err // Return error from gz.Close() if any
	})
}
