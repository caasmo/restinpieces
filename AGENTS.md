## Architecture

### Use of pelletier/v1 and v2

This project uses both `github.com/pelletier/go-toml` (v1) and `github.com/pelletier/go-toml/v2`. The split is deliberate.

**v2** is used everywhere struct marshalling/unmarshalling is needed:
- Framework startup (`restinpieces.go`) — load config from DB
- Config reload (`config/reload.go`) — validate on SIGHUP
- CLI commands: `dump`, `diff`, `log_init`, `app_create` — marshal/unmarshal for normalization

**v1** is used only where a mutable tree API is required:
- `set`, `get`, `paths` commands — dot-path navigation, recursive enumeration, in-place mutation
- Note: `init_command.go` uses v1 for `toml.Marshal` but could switch to v2

**Why not v2 only**: v2 has no tree enumeration API — no `Keys()`, no recursive walk, no `*toml.Tree` type assertion. This makes `get`/`paths` impossible with v2 alone. v2's `unstable/edit` can replace v1 for `set` (better array handling, comment preservation), but `get`/`paths` would need to unmarshal into `map[string]interface{}` and walk that — losing v1's convenience.

**Rule**: new code that only needs marshal/unmarshal uses v2. Code that needs tree mutation or enumeration uses v1. Never mix both in the same file.

### Config: maps over slices

Config structs **MUST NOT** contain slices for collections of items. Use `map[string]T` instead.

**Why**: `ripc config set`, `get`, and `paths` operate on dot-paths like `server.addr`. Maps produce stable paths (`backup_local.files.app_db.source_path`). Slices produce unstable paths (`backup_local.files[2].source_path`) — the index breaks on insertions and deletions. Maps make every path a permanent, addressable key.

### Config: map key rules

- Keys are arbitrary user-chosen labels (e.g. `app_db`, `my_google`), never domain identifiers.
- Domain identifiers belong as struct fields inside the map value (e.g. `OAuth2Provider.Name = "google"`).

Current violations (to be refactored):
- `BackupLocal.Files` is `[]BackupLocalDbFile` — must become `map[string]BackupLocalDbFile`
- `OAuth2Providers` is `map[string]OAuth2Provider` but uses the key as the provider identifier — must move identifier into the struct and make the key a label
