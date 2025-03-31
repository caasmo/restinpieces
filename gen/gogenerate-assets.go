//go:build ignore
// +build ignore

package main

import (
	"bytes" // To buffer modified HTML
	"compress/gzip"
	"encoding/json" // To parse metafile and create manifest
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html" // For HTML parsing
)

// Define a struct to help parse esbuild's metafile output section
type esbuildMetaOutput struct {
	EntryPoint string `json:"entryPoint"`
	// We only need the entryPoint field for mapping
}

// Define a struct for the entire metafile (we only care about outputs)
type esbuildMetafile struct {
	Outputs map[string]esbuildMetaOutput `json:"outputs"`
}

func main() {
	baseDir := flag.String("baseDir", "public", "The base directory containing src and for dist output")
	flag.Parse()

	srcDir := filepath.Join(*baseDir, "src")
	distDir := filepath.Join(*baseDir, "dist")

	log.Printf("Using Base Directory: %s", *baseDir)
	log.Printf("Source Directory: %s", srcDir)
	log.Printf("Distribution Directory: %s", distDir)

	log.Printf("Cleaning directory: %s", distDir)
	if err := os.RemoveAll(distDir); err != nil {
		if !os.IsNotExist(err) {
			log.Fatalf("Failed to clean dist directory %s: %v", distDir, err)
		}
	}

	log.Printf("Ensuring base distribution directory exists: %s", distDir)
	if err := os.MkdirAll(distDir, 0755); err != nil {
		if !os.IsExist(err) {
			log.Fatalf("Failed to create base distribution directory %s: %v", distDir, err)
		}
	}

	// --- Build Steps ---

	// 1. Process JavaScript (with hashing and metafile)
	log.Println("Processing JavaScript...")
	jsResult, err := processJS(srcDir, distDir) // Capture result
	if err != nil {
		log.Fatalf("JS processing failed: %v", err)
	}
	log.Println("JavaScript processing complete.")

	// 2. Process CSS (with hashing and metafile)
	log.Println("Processing CSS...")
	cssResult, err := processCSS(srcDir, distDir) // Capture result
	if err != nil {
		log.Fatalf("CSS processing failed: %v", err)
	}
	log.Println("CSS processing complete.")

	// 3. Generate Manifest
	log.Println("Generating asset manifest...")
	manifest, err := generateManifest(jsResult, cssResult, srcDir, distDir)
	if err != nil {
		log.Fatalf("Manifest generation failed: %v", err)
	}
	log.Println("Asset manifest generated.")

	// 4. Copy HTML files (Source HTML, before rewriting)
	log.Println("Copying HTML files...")
	if err := copyHTML(srcDir, distDir); err != nil {
		log.Fatalf("HTML copy failed: %v", err)
	}
	log.Println("HTML copy complete.")

	// 5. Rewrite HTML Assets (using the manifest)
	log.Println("Rewriting HTML asset paths...")
	if err := rewriteHTMLAssets(distDir, manifest); err != nil {
		log.Fatalf("HTML rewriting failed: %v", err)
	}
	log.Println("HTML asset paths rewritten.")

	// 6. Gzip all final assets in dist
	log.Println("Gzipping assets...")
	if err := gzipAssets(distDir); err != nil {
		log.Fatalf("Gzip failed: %v", err)
	}
	log.Println("Gzipping complete.")
	log.Println("Build finished successfully.")
}

// --- processJS now returns the build result ---
func processJS(srcDir, distDir string) (api.BuildResult, error) { // Return result
	jsSrcDir := filepath.Join(srcDir, "js")
	jsOutDir := filepath.Join(distDir, "js")

	var entryPoints []string
	files, err := os.ReadDir(jsSrcDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("  Skipping JS processing: Source directory %s not found.", jsSrcDir)
			// Return an empty result if skipped
			return api.BuildResult{}, nil
		}
		return api.BuildResult{}, fmt.Errorf("failed to read JS source directory %s: %w", jsSrcDir, err)
	}

	log.Printf("  Scanning for entry points in: %s", jsSrcDir)
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".js") {
			entryPath := filepath.Join(jsSrcDir, file.Name())
			log.Printf("    Found entry point: %s", entryPath)
			entryPoints = append(entryPoints, entryPath)
		} // Skip directories and non-JS files logging (already present)
	}

	if len(entryPoints) == 0 {
		log.Println("  No JavaScript entry points found.")
		return api.BuildResult{}, nil // Return empty result
	}

	log.Printf("  Processing %d entry point(s) with code splitting and hashing.", len(entryPoints))
	log.Printf("  Output directory: %s", jsOutDir)

	result := api.Build(api.BuildOptions{
		EntryPoints: entryPoints,
		Bundle:      true,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Splitting:         true,
		Format:            api.FormatESModule,
		Target:            api.ES2017,
		Platform:          api.PlatformBrowser,
		Outdir:            jsOutDir,
		Write:             true,
		LogLevel:          api.LogLevelInfo,
		EntryNames:        "[name]-[hash]", // Hashing for entry points
		ChunkNames:        "[name]-[hash]", // Hashing for shared chunks (all in jsOutDir)
		Metafile:          true,            // Generate metafile for mapping
	})

	// Error and warning handling (remains the same detailed logging)
	// ... (Error logging as before) ...
    hasErrors := len(result.Errors) > 0
    hasWarnings := len(result.Warnings) > 0
    if hasErrors {
		log.Printf("  ESBuild JS encountered errors:")
		for _, err := range result.Errors { // Use same detailed logging as JS
			location := ""
			lineText := ""
			if err.Location != nil {
				location = fmt.Sprintf(" (%s:%d:%d)", err.Location.File, err.Location.Line, err.Location.Column)
                lineText = fmt.Sprintf("\n      > %s", err.Location.LineText)
			}
			log.Printf("    Error: %s%s%s", err.Text, location, lineText)
		}
	}
	if hasWarnings {
		log.Printf("  ESBuild JS generated warnings:")
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
    if hasErrors {
        return result, fmt.Errorf("ESBuild JS failed with %d errors", len(result.Errors)) // Return result even on error
    }


	log.Printf("  Successfully processed JavaScript to %s", jsOutDir)
	return result, nil // Return result on success
}

// --- processCSS now returns the build result ---
func processCSS(srcDir, distDir string) (api.BuildResult, error) { // Return result
	entryPoint := filepath.Join(srcDir, "css", "restinpieces.css")
	cssOutDir := filepath.Join(distDir, "css") // Define output dir

	if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
		log.Printf("  Skipping CSS processing: Entry point %s not found.", entryPoint)
		return api.BuildResult{}, nil // Return empty result
	} else if err != nil {
		return api.BuildResult{}, fmt.Errorf("failed to check CSS entry point %s: %w", entryPoint, err)
	}

	log.Printf("  Entry point: %s", entryPoint)
	log.Printf("  Output directory: %s", cssOutDir)

	result := api.Build(api.BuildOptions{
		EntryPoints:       []string{entryPoint},
		Bundle:            true,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Outdir:            cssOutDir,       // Use Outdir
		Write:             true,
		LogLevel:          api.LogLevelInfo,
		EntryNames:        "[name]-[hash]", // Hashing for CSS entry points
		Metafile:          true,            // Generate metafile
	})

	// Error/Warning handling... (identical to previous version / processJS)
    hasErrors := len(result.Errors) > 0
    hasWarnings := len(result.Warnings) > 0
	if hasErrors {
		log.Printf("  ESBuild CSS encountered errors:")
		for _, err := range result.Errors {
			location := ""
			lineText := ""
			if err.Location != nil {
				location = fmt.Sprintf(" (%s:%d:%d)", err.Location.File, err.Location.Line, err.Location.Column)
                lineText = fmt.Sprintf("\n      > %s", err.Location.LineText)
			}
			log.Printf("    Error: %s%s%s", err.Text, location, lineText)
		}
	}
	if hasWarnings {
		log.Printf("  ESBuild CSS generated warnings:")
		for _, warn := range result.Warnings {
			location := ""
            lineText := ""
			if warn.Location != nil {
				location = fmt.Sprintf(" (%s:%d:%d)", warn.Location.File, warn.Location.Line, warn.Location.Column)
                lineText = fmt.Sprintf("\n      > %s", warn.Location.LineText)
			}
			log.Printf("    Warning: %s%s%s", warn.Text, location, lineText)
		}
	}
    if hasErrors {
        return result, fmt.Errorf("ESBuild CSS failed with %d errors", len(result.Errors))
    }


	log.Printf("  Successfully processed CSS to %s", cssOutDir)
	return result, nil // Return result on success
}

func generateManifest(jsResult, cssResult api.BuildResult, srcDir, distDir string) (map[string]string, error) {
	manifest := make(map[string]string)

	// Helper function to process metafile outputs
	processMeta := func(metafile string, metaType string) error {
		if metafile == "" {
			log.Printf("  Skipping %s metafile processing: No metafile generated.", metaType)
			return nil
		}

		var meta esbuildMetafile
		if err := json.Unmarshal([]byte(metafile), &meta); err != nil {
			return fmt.Errorf("failed to parse %s metafile: %w", metaType, err)
		}

		log.Printf("  Processing %s metafile (%d outputs)", metaType, len(meta.Outputs))
		for hashedPath, output := range meta.Outputs {
			// We only care about outputs that correspond to an original entry point
			if output.EntryPoint == "" {
				continue // Skip chunks or other non-entry outputs for the manifest
			}

			// --- Calculate Manifest Key ---
			// Key: Relative path from srcDir (e.g., "js/dashboard.js")
			relSrcPath, err := filepath.Rel(srcDir, output.EntryPoint)
			if err != nil {
				log.Printf("    Warning: Could not make source path relative %q: %v", output.EntryPoint, err)
				continue
			}
			manifestKey := filepath.ToSlash(relSrcPath) // Use forward slashes

			// --- Calculate Manifest Value ---
			// Value: Path relative to distDir, made absolute from web root (e.g., "/js/dashboard-a1b2c3d4.js")

			// 1. Make the hashed path relative to the distribution directory
			//    hashedPath might be "public/dist/js/file-HASH.js"
			//    distDir might be "public/dist"
			//    We want "js/file-HASH.js"
			relHashedPath, err := filepath.Rel(distDir, hashedPath)
			if err != nil {
				log.Printf("    Warning: Could not make hashed path %q relative to dist dir %q: %v", hashedPath, distDir, err)
				continue // Skip if we can't make it relative
			}

			// 2. Prepend "/" and ensure forward slashes for the web path
			manifestValue := "/" + filepath.ToSlash(relHashedPath)

			log.Printf("    Mapping: %s -> %s", manifestKey, manifestValue)
			manifest[manifestKey] = manifestValue
		}
		return nil
	}

	// Process JS Metafile
	if err := processMeta(jsResult.Metafile, "JS"); err != nil {
		return nil, err // Propagate error
	}

	// Process CSS Metafile
	if err := processMeta(cssResult.Metafile, "CSS"); err != nil {
		return nil, err // Propagate error
	}

	// Write manifest file (if any mappings were generated)
	if len(manifest) > 0 {
		manifestPath := filepath.Join(distDir, "manifest.json")
		manifestData, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal manifest: %w", err)
		}
		if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write manifest file %s: %w", manifestPath, err)
		}
		log.Printf("  Manifest written to %s", manifestPath)
	} else {
		log.Println("  No manifest entries generated, skipping manifest file write.")
	}


	return manifest, nil
}

// --- NEW function to generate manifest.json ---
func generateManifest2(jsResult, cssResult api.BuildResult, srcDir, distDir string) (map[string]string, error) {
	manifest := make(map[string]string)

	// Process JS Metafile
	if jsResult.Metafile != "" {
		var jsMeta esbuildMetafile
		if err := json.Unmarshal([]byte(jsResult.Metafile), &jsMeta); err != nil {
			return nil, fmt.Errorf("failed to parse JS metafile: %w", err)
		}
		log.Printf("  Processing JS metafile (%d outputs)", len(jsMeta.Outputs))
		for hashedPath, output := range jsMeta.Outputs {
			if output.EntryPoint != "" {
				// Key: Relative path from srcDir (e.g., "js/dashboard.js")
				relSrcPath, err := filepath.Rel(srcDir, output.EntryPoint)
				if err != nil {
					log.Printf("    Warning: Could not make source path relative %q: %v", output.EntryPoint, err)
					continue
				}
				manifestKey := filepath.ToSlash(relSrcPath) // Use forward slashes

				// Value: Absolute path from web root (e.g., "/dist/js/dashboard-a1b2c3d4.js")
				absDistPath := "/" + filepath.ToSlash(hashedPath) // Ensure leading slash and forward slashes

				log.Printf("    Mapping: %s -> %s", manifestKey, absDistPath)
				manifest[manifestKey] = absDistPath
			}
		}
	}

	// Process CSS Metafile
	if cssResult.Metafile != "" {
		var cssMeta esbuildMetafile
		if err := json.Unmarshal([]byte(cssResult.Metafile), &cssMeta); err != nil {
			return nil, fmt.Errorf("failed to parse CSS metafile: %w", err)
		}
        log.Printf("  Processing CSS metafile (%d outputs)", len(cssMeta.Outputs))
		for hashedPath, output := range cssMeta.Outputs {
			if output.EntryPoint != "" {
				relSrcPath, err := filepath.Rel(srcDir, output.EntryPoint)
				if err != nil {
					log.Printf("    Warning: Could not make source path relative %q: %v", output.EntryPoint, err)
					continue
				}
				manifestKey := filepath.ToSlash(relSrcPath)
				absDistPath := "/" + filepath.ToSlash(hashedPath)
				log.Printf("    Mapping: %s -> %s", manifestKey, absDistPath)
				manifest[manifestKey] = absDistPath
			}
		}
	}

	// Write manifest file
	manifestPath := filepath.Join(distDir, "manifest.json")
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write manifest file %s: %w", manifestPath, err)
	}
	log.Printf("  Manifest written to %s", manifestPath)

	return manifest, nil
}

// copyHTML function remains the same (copies source files)
func copyHTML(srcDir, distDir string) error {
	log.Printf("  Walking source directory: %s", srcDir)
	foundHTML := false
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
        // ... (Error handling and directory skipping logic remains the same) ...
        if err != nil {
			log.Printf("  Error accessing path %q: %v", path, err)
			return err
		}
		if info.IsDir() {
            baseName := filepath.Base(path)
            if path != srcDir && (baseName == "js" || baseName == "css") {
                 log.Printf("    Skipping walk into processed directory: %s", path)
                 return filepath.SkipDir
            }
			return nil
		}

		// For this function, we ONLY copy HTML. Other assets are handled by esbuild or need specific copying logic.
		if filepath.Ext(path) != ".html" {
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
        log.Printf("  Successfully copied HTML source files.")
    }
	return err
}


// --- NEW function to rewrite asset paths in HTML files ---
func rewriteHTMLAssets(distDir string, manifest map[string]string) error {
	log.Printf("  Walking dist directory for HTML files: %s", distDir)

    // We need this mapping to normalize paths found in HTML src/href
    // to match the manifest keys (which are relative to srcDir).
    // For example, HTML might have "/dist/js/dashboard.js" or "js/dashboard.js",
    // but the manifest key is "js/dashboard.js".
    normalizePrefixes := []string{
        "/" + filepath.Base(distDir) + "/", // e.g. "/dist/"
        filepath.Base(distDir) + "/",      // e.g. "dist/"
        "/",                              // leading slash
    }

	return filepath.Walk(distDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".html" {
			return nil // Only process HTML files
		}

		log.Printf("    Processing HTML file: %s", path)

		// Read the HTML file content
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			log.Printf("      Error reading HTML file %s: %v", path, err)
			return err // Skip this file on read error
		}
		contentReader := bytes.NewReader(contentBytes)

		// Parse the HTML
		doc, err := html.Parse(contentReader)
		if err != nil {
			log.Printf("      Error parsing HTML file %s: %v", path, err)
			return err // Skip this file on parse error
		}

		var changed bool
		var traverse func(*html.Node)
		traverse = func(n *html.Node) {
			if n.Type == html.ElementNode {
				var attrKey string
				if n.Data == "script" {
					attrKey = "src"
				} else if n.Data == "link" {
                    // Ensure it's a stylesheet link
                    isStylesheet := false
                    for _, a := range n.Attr {
                        if a.Key == "rel" && strings.ToLower(a.Val) == "stylesheet" {
                            isStylesheet = true
                            break
                        }
                    }
                    if isStylesheet {
					    attrKey = "href"
                    }
				}

				if attrKey != "" {
                    originalValue := ""
                    attrIndex := -1
					for i, a := range n.Attr {
						if a.Key == attrKey {
                            originalValue = a.Val
                            attrIndex = i
							break
						}
					}

                    if originalValue != "" && attrIndex != -1 {
                        // Normalize the path found in HTML to match manifest key format
                        lookupKey := strings.TrimSpace(originalValue)
                        for _, prefix := range normalizePrefixes {
                            lookupKey = strings.TrimPrefix(lookupKey, prefix)
                        }
                        lookupKey = filepath.ToSlash(lookupKey) // Ensure forward slashes for lookup

                        // Check manifest
                        if hashedPath, ok := manifest[lookupKey]; ok {
                            log.Printf("      Replacing %s=%q with %q", attrKey, originalValue, hashedPath)
                            n.Attr[attrIndex].Val = hashedPath // Update the attribute value
                            changed = true
                        }
                    }
				}
			}

			// Traverse children and siblings
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				traverse(c)
			}
		}

		traverse(doc)

		// If changes were made, write the modified HTML back
		if changed {
			var buf bytes.Buffer
			if err := html.Render(&buf, doc); err != nil {
				log.Printf("      Error rendering modified HTML for %s: %v", path, err)
				return err // Skip writing on render error
			}

			if err := os.WriteFile(path, buf.Bytes(), info.Mode()); err != nil { // Use original file mode
				log.Printf("      Error writing modified HTML file %s: %v", path, err)
				return err
			}
			log.Printf("    Successfully rewrote assets in %s", path)
		} else {
            log.Printf("    No replaceable asset paths found in %s", path)
        }

		return nil
	})
}


// gzipAssets function remains the same (gzips everything in dist)
func gzipAssets(distDir string) error {
	log.Printf("  Walking distribution directory for gzipping: %s", distDir)
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
        defer out.Close()

		gz := gzip.NewWriter(out)
		_, copyErr := io.Copy(gz, in)
        closeErr := gz.Close() // Close before checking copyErr

		if copyErr != nil {
			log.Printf("  Error gzipping data for %q: %v", path, copyErr)
			os.Remove(gzipPath)
			return copyErr
		}
		if closeErr != nil {
			log.Printf("  Error finalizing gzip stream for %q: %v", gzipPath, closeErr)
			os.Remove(gzipPath)
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
