//go:build ignore
// +build ignore

package main

import (
	"compress/gzip"
	"flag"
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

	// Subdirectories to create within dist - ADD "css" here
	createDirs := []string{"js", "css"}
	for _, dir := range createDirs {
		targetDir := filepath.Join(distDir, dir)
		log.Printf("Creating directory: %s", targetDir)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", targetDir, err)
		}
	}

	// --- Build Steps ---

	// 1. Process JavaScript
	log.Println("Processing JavaScript...")
	if err := processJS(srcDir, distDir); err != nil {
		log.Fatalf("JS processing failed: %v", err)
	}
	log.Println("JavaScript processing complete.")

	// 2. Process CSS - NEW STEP
	log.Println("Processing CSS...")
	if err := processCSS(srcDir, distDir); err != nil {
		log.Fatalf("CSS processing failed: %v", err)
	}
	log.Println("CSS processing complete.")

	// 3. Copy HTML files
	log.Println("Copying HTML files...")
	if err := copyHTML(srcDir, distDir); err != nil {
		log.Fatalf("HTML copy failed: %v", err)
	}
	log.Println("HTML copy complete.")

	// 4. Gzip all assets
	log.Println("Gzipping assets...")
	if err := gzipAssets(distDir); err != nil {
		log.Fatalf("Gzip failed: %v", err)
	}
	log.Println("Gzipping complete.")
	log.Println("Build finished successfully.")
}

func processJS(srcDir, distDir string) error {
	// Assuming the main JS entry point is named consistently
	entryPoint := filepath.Join(srcDir, "js", "restinpieces.js")
	outFile := filepath.Join(distDir, "js", "restinpieces.js")

	// Check if entry point exists
	if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
		log.Printf("  Skipping JS processing: Entry point %s not found.", entryPoint)
		return nil // Not a fatal error if entry point doesn't exist
	} else if err != nil {
		return fmt.Errorf("failed to check JS entry point %s: %w", entryPoint, err)
	}


	log.Printf("  Entry point: %s", entryPoint)
	log.Printf("  Output file: %s", outFile)

	result := api.Build(api.BuildOptions{
		EntryPoints:       []string{entryPoint},
		Bundle:            true,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Drop:              api.DropConsole, // Keep console drop specific to JS
		Format:            api.FormatESModule,
		Target:            api.ES2017, // Target for modern browsers
		Platform:          api.PlatformBrowser,
		Outfile:           outFile,
		Write:             true,
		LogLevel:          api.LogLevelInfo, // Show warnings/info from esbuild
	})

	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			// Provide more context for esbuild errors
			location := ""
			if err.Location != nil {
				location = fmt.Sprintf(" (%s:%d:%d)", err.Location.File, err.Location.Line, err.Location.Column)
			}
			log.Printf("  ESBuild JS Error: %s%s\n    %s", err.Text, location, err.Location.LineText)
		}
		return fmt.Errorf("ESBuild JS failed with %d errors", len(result.Errors))
	}
	if len(result.Warnings) > 0 {
		for _, warn := range result.Warnings {
			location := ""
			if warn.Location != nil {
				location = fmt.Sprintf(" (%s:%d:%d)", warn.Location.File, warn.Location.Line, warn.Location.Column)
			}
			log.Printf("  ESBuild JS Warning: %s%s\n    %s", warn.Text, location, warn.Location.LineText)
		}
	}
	log.Printf("  Successfully processed %s", outFile)
	return nil
}

// New function to process CSS
func processCSS(srcDir, distDir string) error {
	// Assuming the main CSS entry point is named consistently (e.g., style.css or main.css)
	// Let's match the JS naming convention for this example.
	entryPoint := filepath.Join(srcDir, "css", "restinpieces.css")
	outFile := filepath.Join(distDir, "css", "restinpieces.css")

	// Check if entry point exists
	if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
		log.Printf("  Skipping CSS processing: Entry point %s not found.", entryPoint)
		return nil // Not a fatal error if entry point doesn't exist
	} else if err != nil {
		return fmt.Errorf("failed to check CSS entry point %s: %w", entryPoint, err)
	}

	log.Printf("  Entry point: %s", entryPoint)
	log.Printf("  Output file: %s", outFile)

	result := api.Build(api.BuildOptions{
		EntryPoints:       []string{entryPoint},
		Bundle:            true, // Bundle @import statements
		MinifyWhitespace:  true, // Remove unnecessary whitespace
		MinifyIdentifiers: true, // Shorten identifiers (like animation names, CSS variables if safe)
		MinifySyntax:      true, // Use shorter syntax equivalents (e.g., colors)
		// Target: Use default or specify modern browser targets if needed, affects CSS nesting etc.
		// Target: []string{"chrome90", "firefox88", "safari14", "edge90"},
		// Loader: map[string]api.Loader{".css": api.LoaderCSS}, // Usually inferred, but can be explicit
		Outfile:   outFile,
		Write:     true,
		LogLevel:  api.LogLevelInfo, // Show warnings/info from esbuild
	})

	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			location := ""
			if err.Location != nil {
				location = fmt.Sprintf(" (%s:%d:%d)", err.Location.File, err.Location.Line, err.Location.Column)
			}
			log.Printf("  ESBuild CSS Error: %s%s\n    %s", err.Text, location, err.Location.LineText)
		}
		return fmt.Errorf("ESBuild CSS failed with %d errors", len(result.Errors))
	}
	if len(result.Warnings) > 0 {
		for _, warn := range result.Warnings {
			location := ""
			if warn.Location != nil {
				location = fmt.Sprintf(" (%s:%d:%d)", warn.Location.File, warn.Location.Line, warn.Location.Column)
			}
			log.Printf("  ESBuild CSS Warning: %s%s\n    %s", warn.Text, location, warn.Location.LineText)
		}
	}
	log.Printf("  Successfully processed %s", outFile)
	return nil
}

func copyHTML(srcDir, distDir string) error {
	log.Printf("  Walking source directory: %s", srcDir)
	foundHTML := false // Flag to track if any HTML was found
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
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
		foundHTML = true // Mark that we found at least one HTML file

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

	if err == nil && !foundHTML {
		log.Println("  No HTML files found to copy.")
	}
	return err
}

func gzipAssets(distDir string) error {
	log.Printf("  Walking distribution directory: %s", distDir)
	var gzippedCount int
	err := filepath.Walk(distDir, func(path string, info os.FileInfo, err error) error {
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
		defer out.Close() // Ensure file handle is closed

		// Create gzip writer with default compression
		gz := gzip.NewWriter(out)
		// Consider using gzip.NewWriterLevel for different compression levels if needed
		// gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
		// if err != nil { log.Printf("Error creating gzip writer for %q: %v", gzipPath, err); return err }


		// Copy content
		_, copyErr := io.Copy(gz, in)
		// Crucially, close the gzip writer *before* checking the copy error.
		// Closing flushes buffers and writes the gzip footer.
		closeErr := gz.Close()

		if copyErr != nil {
			log.Printf("  Error gzipping data for %q: %v", path, copyErr)
			// Clean up potentially partially written .gz file on error
			out.Close()         // Close file handle first
			os.Remove(gzipPath) // Attempt removal
			return copyErr      // Return the copy error
		}

		if closeErr != nil {
			log.Printf("  Error finalizing gzip stream for %q: %v", gzipPath, closeErr)
			// Clean up potentially corrupt .gz file on close error
			out.Close()         // Close file handle
			os.Remove(gzipPath) // Attempt removal
			return closeErr     // Return the close error
		}
		gzippedCount++
		return nil // Success for this file
	})

	if err == nil {
		log.Printf("  Successfully gzipped %d file(s).", gzippedCount)
	}
	return err // Return error from filepath.Walk if any occurred
}
