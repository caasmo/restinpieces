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

**Why**: `ripc config set`, `get`, and `paths` operate on dot-paths like `server.addr`. Maps produce stable paths (`backup_local.files.app_db.source_path`). Slices produce unstable paths (`backup_local.files[2].source_path`) — the index breaks on insertions and deletions. Maps make every path a permanent, addressable key.

### Config: map key rules

- Keys are arbitrary user-chosen labels (e.g. `app_db`, `my_google`), never domain identifiers.
- Domain identifiers belong as struct fields inside the map value (e.g. `OAuth2Provider.Name = "google"`).

Current violations (to be refactored):
- `BackupLocal.Files` is `[]BackupLocalDbFile` — must become `map[string]BackupLocalDbFile`
- `OAuth2Providers` is `map[string]OAuth2Provider` but uses the key as the provider identifier — must move identifier into the struct and make the key a label
