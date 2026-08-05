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

// LatestGlob is the shell glob matching every latest hardlink in backupDir.
// rsync clients pass it as the remote file argument to sync all configured
// databases in a single run, e.g.:
//
//	rsync --server --sender ... . <backupDir>/latest-*.db
//
// The remote login shell expands the glob before rsync parses its arguments
// (gokrazy/rsync derives the sender file list from argv), so the client needs
// no directory listing. The trailing ".db" is deliberate: it excludes the
// transient "<name>.db.tmp" hardlink that linkLatest creates between
// link(2) and rename(2), and timestamped snapshots never match because they
// start with the backupID, not "latest-".
//
// A glob with zero matches is expanded by the shell to the literal pattern;
// the sender then transfers zero files and the rsync protocol completes
// without error. Clients must treat "no files transferred" as a failure, and
// the SSH account needs a normal POSIX login shell for the expansion to work.
const LatestGlob = "latest-*.db"
