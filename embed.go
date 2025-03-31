//go:generate go run gen/gogenerate-assets.go -baseDir static

package restinpieces

import "embed"

//go:embed static/dist/*
var EmbeddedAssets embed.FS // move to embed.go
