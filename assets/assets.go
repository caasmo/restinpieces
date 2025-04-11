package assets

import _ "embed"

// MaintenancePageGzipped holds the pre-gzipped content of the maintenance page.
// Ensure assets/maintenance.html is gzipped to assets/maintenance.html.gz before building.
//go:embed maintenance.html.gz
var MaintenancePageGzipped []byte
