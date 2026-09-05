## Architecture

### Use of pelletier/v1 and v2

This project uses both `github.com/pelletier/go-toml` (v1) and `github.com/pelletier/go-toml/v2`.

**Framework** (`restinpieces.go`, `config/reload.go`, etc.) uses v2 exclusively. It only needs `Unmarshal` to load config from the database — no tree navigation required.

**ripc CLI** uses both:
- v1 for `set`, `get`, `paths` — these need `tree.Keys()` and recursive tree-walk to enumerate and mutate TOML keys. v2 has no equivalent.
- v2 for `dump`, `diff`, `log_init`, `app_create` — these only marshal/unmarshal structs.

**Rule**: new code uses v2 unless it needs to enumerate or mutate TOML tree keys. Never import both versions in the same file.

#### v2 unstable/edit

v2's `unstable/edit` sub-package now provides `Set`, `Has`, `Get`, `Delete` with comment preservation — the tree-mutation API v2 was missing. Once it stabilizes into a tagged release, the remaining v1 code (`set`, `get`, `paths`) will migrate to it.

### Config: maps over slices

Config structs **MUST NOT** contain slices for collections of items. Use `map[string]T` instead.

**Why**: `ripc set`, `get`, and `paths` operate on dot-paths like `server.addr`. Maps produce stable paths (`backup.files.app_db.source_path`). Slices produce unstable paths (`backup.files[2].source_path`) — the index breaks on insertions and deletions. Maps make every path a permanent, addressable key.

### Config: map key rules

- Keys are arbitrary user-chosen labels (e.g. `app_db`, `my_google`), never domain identifiers.
- Domain identifiers belong as struct fields inside the map value (e.g. `OAuth2Provider.Name = "google"`).

Current violations (to be refactored):
- `OAuth2Providers` is `map[string]OAuth2Provider` but uses the key as the provider identifier — must move identifier into the struct and make the key a label

### Config: path fields

All config path fields (e.g. `dest_path`, `source_path`, `db_path`, `public_dir`) are absolute paths or relative paths resolved against the binary's current working directory (CWD). Absolute paths are used as-is. No path in config is ever resolved against a config file location — there is no config file, only a database. When deployed via the canonical systemd service, the CWD is `/home/<app>`, so relative paths typically start with `data/`.

An empty path `""` is the zero value and means **deactivated**, never the CWD: `backup.files.<key>.source_path = ""` deactivates that file entry (and an empty `backup.files` map deactivates the backup feature). Validation accepts `""` and requires non-empty path fields to resolve to the required kind (`dest_path` to an existing directory, `source_path` to an existing file).

### CLI help framework: shared copy-paste file

`cmd/ripc/help.go` is the reference implementation of the project's
data-driven CLI help framework: types `Spec`, `OptSpec`, `ArgSpec`,
`Subcommand`, `SubcommandGroup`, and the shared renderer `Spec.Print`. It is
**stdlib-only** and must stay that way so it remains copyable.

**Rule**: other CLIs copy `cmd/ripc/help.go` **verbatim** — no forks, no
package variants, no rewrites. ripc is the most modern revision of this
framework across all repos; when `help.go` changes here, sync the new version
to the other CLIs in the same change.

Help is data-driven: every command declares a `Spec` literal next to its
command file, and `Spec.Opt(name)` is the single source of truth for flag
defaults and usage text at flag-registration time.

### Benchmarks: component-scenario naming

A benchmark name starts with the component being tested, then the scenario,
separated by underscores.

- Good: `BenchmarkCache_GetHit`, `BenchmarkAuthenticator_HappyPath`,
  `BenchmarkBlockIp_Process`
- Bad: `GetConfig_Serial` — the component is missing, so you can't tell what
  is being tested without opening the file

For concurrency variants, append `_Serial` or `_Parallel` to the scenario,
like `BenchmarkLog_InsertBatch_Serial` and `BenchmarkQueue_Claim_Parallel`.

Benchmark names appear in the comparison reports. Renaming one loses its
history, so once a benchmark is released, its name stays.

### Code verification

Before merging, all four must pass:

```sh
go build ./...
go vet ./...
golangci-lint run ./...
gofmt -l .
```

`gofmt -l .` must print nothing.

### Tag a release

1. Run the generating-release-notes skill to write `RELEASE_<short-hash>.md`.
2. Tag verbatim (preserves markdown `#` headers): `git tag -a vX.Y.Z -F RELEASE_<short-hash>.md --cleanup=verbatim`
3. Verify: `git cat-file -p vX.Y.Z`
