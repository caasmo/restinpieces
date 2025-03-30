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
	"strings" // Import strings package

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

	// Clean dist directory
	log.Printf("Cleaning directory: %s", distDir)
	if err := os.RemoveAll(distDir); err != nil {
		if !os.IsNotExist(err) {
			log.Fatalf("Failed to clean dist directory %s: %v", distDir, err)
		}
	}

	// Ensure base distribution directory exists
	log.Printf("Ensuring base distribution directory exists: %s", distDir)
	if err := os.MkdirAll(distDir, 0755); err != nil {
		if !os.IsExist(err) { // Allow existing directory
			log.Fatalf("Failed to create base distribution directory %s: %v", distDir, err)
		}
	}


	// --- Build Steps ---

	// 1. Process JavaScript (Now with dynamic entry points and splitting)
	log.Println("Processing JavaScript...")
	if err := processJS(srcDir, distDir); err != nil {
		log.Fatalf("JS processing failed: %v", err)
	}
	log.Println("JavaScript processing complete.")

	// 2. Process CSS
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

// --- Refactored processJS ---
func processJS(srcDir, distDir string) error {
	jsSrcDir := filepath.Join(srcDir, "js")
	jsOutDir := filepath.Join(distDir, "js") // Define the output directory for JS

	// Find entry points: .js files directly in jsSrcDir, excluding subdirectories
	var entryPoints []string
	files, err := os.ReadDir(jsSrcDir)
	if err != nil {
		// If the js source directory doesn't exist, just skip JS processing
		if os.IsNotExist(err) {
			log.Printf("  Skipping JS processing: Source directory %s not found.", jsSrcDir)
			return nil
		}
		return fmt.Errorf("failed to read JS source directory %s: %w", jsSrcDir, err)
	}

	log.Printf("  Scanning for entry points in: %s", jsSrcDir)
	for _, file := range files {
		// Check if it's a file (not a directory) and ends with .js
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".js") {
			entryPath := filepath.Join(jsSrcDir, file.Name())
			log.Printf("    Found entry point: %s", entryPath)
			entryPoints = append(entryPoints, entryPath)
		} else if file.IsDir() {
            log.Printf("    Skipping directory: %s", file.Name())
        } else {
            log.Printf("    Skipping non-JS file: %s", file.Name())
        }
	}

	// If no entry points found, we're done for JS
	if len(entryPoints) == 0 {
		log.Println("  No JavaScript entry points found.")
		return nil
	}

	log.Printf("  Processing %d entry point(s) with code splitting.", len(entryPoints))
	log.Printf("  Output directory: %s", jsOutDir)


	// Note: With splitting=true, esbuild handles creating jsOutDir if needed.
	result := api.Build(api.BuildOptions{
		EntryPoints:       entryPoints,       // Use the dynamic list
		Bundle:            true,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Splitting:         true,              // Enable code splitting
		Format:            api.FormatESModule, // Must be ESM for splitting
		Target:            api.ES2017,
		Platform:          api.PlatformBrowser,
		Outdir:            jsOutDir,         // Use Outdir, not Outfile
		Write:             true,
		LogLevel:          api.LogLevelInfo,
		// Chunks will be named automatically by esbuild for now
		// We'll add entryNames/chunkNames later for versioning/hashing
	})

	// Error and warning handling
    hasErrors := len(result.Errors) > 0
    hasWarnings := len(result.Warnings) > 0

	if hasErrors {
		log.Printf("  ESBuild JS encountered errors:")
		for _, err := range result.Errors {
			location := ""
			lineText := ""
			if err.Location != nil {
				location = fmt.Sprintf(" (%s:%d:%d)", err.Location.File, err.Location.Line, err.Location.Column)
                lineText = fmt.Sprintf("\n      > %s", err.Location.LineText) // Add line context
			}
			log.Printf("    Error: %s%s%s", err.Text, location, lineText)
		}
	}
	if hasWarnings {
		log.Printf("  ESBuild JS generated warnings:")
		for _, warn := range result.Warnings {
			location := ""
            lineText := ""
			if warn.Location != nil {
				location = fmt.Sprintf(" (%s:%d:%d)", warn.Location.File, warn.Location.Line, warn.Location.Column)
                lineText = fmt.Sprintf("\n      > %s", warn.Location.LineText) // Add line context
			}
			log.Printf("    Warning: %s%s%s", warn.Text, location, lineText)
		}
	}

	if hasErrors {
		return fmt.Errorf("ESBuild JS failed with %d errors", len(result.Errors))
	}

	log.Printf("  Successfully processed JavaScript to %s", jsOutDir)
	return nil
}


// processCSS function remains the same
func processCSS(srcDir, distDir string) error {
	entryPoint := filepath.Join(srcDir, "css", "restinpieces.css")
	outFile := filepath.Join(distDir, "css", "restinpieces.css") // esbuild will create dist/css if needed

	if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
		log.Printf("  Skipping CSS processing: Entry point %s not found.", entryPoint)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to check CSS entry point %s: %w", entryPoint, err)
	}

	log.Printf("  Entry point: %s", entryPoint)
	log.Printf("  Output file: %s", outFile)

	result := api.Build(api.BuildOptions{
		EntryPoints:       []string{entryPoint},
		Bundle:            true,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Outfile:           outFile,
		Write:             true,
		LogLevel:          api.LogLevelInfo,
	})

	// Error/Warning handling... (identical to previous version)
    hasErrors := len(result.Errors) > 0
    hasWarnings := len(result.Warnings) > 0
	if hasErrors {
		log.Printf("  ESBuild CSS encountered errors:")
		for _, err := range result.Errors { // Use same detailed logging as JS
			location := ""
			lineText := ""
			if err.Location != nil {
				location = fmt.Sprintf(" (%s:%d:%d)", err.Location.File, err.Location.Line, err.Location.Column)
                lineText = fmt.Sprintf("\n      > %s", err.Location.LineText)
			}
			log.Printf("    Error: %s%s%s", err.Text, location, lineText)
		}
        return fmt.Errorf("ESBuild CSS failed with %d errors", len(result.Errors))
	}
	if hasWarnings {
		log.Printf("  ESBuild CSS generated warnings:")
		for _, warn := range result.Warnings { // Use same detailed logging as JS
			location := ""
            lineText := ""
			if warn.Location != nil {
				location = fmt.Sprintf(" (%s:%d:%d)", warn.Location.File, warn.Location.Line, warn.Location.Column)
                lineText = fmt.Sprintf("\n      > %s", warn.Location.LineText)
			}
			log.Printf("    Warning: %s%s%s", warn.Text, location, lineText)
		}
	}
	log.Printf("  Successfully processed %s", outFile)
	return nil
}


// copyHTML function remains the same
func copyHTML(srcDir, distDir string) error {
	log.Printf("  Walking source directory: %s", srcDir)
	foundHTML := false
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("  Error accessing path %q: %v", path, err)
			return err
		}
		if info.IsDir() {
			// Don't descend into js or css subdirectories within src
            // as their contents are processed, not copied directly.
            // Allow descending into other directories if needed (e.g., src/img)
            baseName := filepath.Base(path)
            if path != srcDir && (baseName == "js" || baseName == "css") {
                 log.Printf("    Skipping walk into processed directory: %s", path)
                 return filepath.SkipDir
            }
			return nil // Continue walking other directories
		}
		// Only copy top-level HTML files or files in other subdirs (e.g. img)
		if filepath.Ext(path) != ".html" {
             // Optionally copy other static assets here too (e.g. images, fonts)
             // Example: if filepath.Ext(path) != ".html" && filepath.Ext(path) != ".png" { return nil }
			return nil
		}
		foundHTML = true

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			log.Printf("  Error calculating relative path for %q from %q: %v", path, srcDir, err)
			return err
		}
		dest := filepath.Join(distDir, relPath)
		destParentDir := filepath.Dir(dest)

		if err := os.MkdirAll(destParentDir, 0755); err != nil {
			log.Printf("  Error creating destination directory %q: %v", destParentDir, err)
			return err
		}

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
		return err
	})

	if err == nil && !foundHTML {
		log.Println("  No HTML files found to copy.")
	} else if err == nil {
        log.Printf("  Successfully copied HTML files.")
    }
	return err
}


// gzipAssets function remains the same
func gzipAssets(distDir string) error {
	log.Printf("  Walking distribution directory: %s", distDir)
	var gzippedCount int
	err := filepath.Walk(distDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("  Error accessing path %q: %v", path, err)
			return err
		}
		if info.IsDir() || filepath.Ext(path) == ".gz" {
			return nil
		}
		gzipPath := path + ".gz"
		log.Printf("  Gzipping %s to %s", path, gzipPath)
		in, err := os.Open(path)
		if err != nil {
			log.Printf("  Error opening file %q for gzipping: %v", path, err)
			return err
		}
		defer in.Close()
		out, err := os.Create(gzipPath)
		if err != nil {
			log.Printf("  Error creating gzip file %q: %v", gzipPath, err)
			return err
		}
		// Ensure file handle is closed even if gzip writer fails
        defer out.Close()

		gz := gzip.NewWriter(out)
		_, copyErr := io.Copy(gz, in)
		// Close the gzip writer *before* checking errors to flush buffers/write footer
        closeErr := gz.Close()

		if copyErr != nil {
			log.Printf("  Error gzipping data for %q: %v", path, copyErr)
			os.Remove(gzipPath) // Attempt removal on copy error
			return copyErr
		}
		if closeErr != nil {
			log.Printf("  Error finalizing gzip stream for %q: %v", gzipPath, closeErr)
			os.Remove(gzipPath) // Attempt removal on close error
			return closeErr
		}
		gzippedCount++
		return nil
	})
	if err == nil {
		log.Printf("  Successfully gzipped %d file(s).", gzippedCount)
	}
	return err
}
