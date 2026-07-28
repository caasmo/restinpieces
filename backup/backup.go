// Package backup provides the naming convention contract shared between
// the backup server handler and the backup pull client.
package backup

// LatestFmt is the format template for the stable hardlink filename.
// Use with fmt.Sprintf(LatestFmt, dbName) to construct the path.
//
// Example: fmt.Sprintf(LatestFmt, "app.db") produces "latest-app.db".
//
// Both the server handler and the rsync client use this constant to
// construct and discover the stable reference path.
const LatestFmt = "latest-%s"
