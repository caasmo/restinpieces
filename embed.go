//go:generate go run gen/gogenerate-assets.go -baseDir public

package restinpieces

import "embed"

//go:embed public/dist/*
var EmbeddedAssets embed.FS // move to embed.go
