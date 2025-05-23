# ConfigStore - Secure Configuration Management for RestInPieces

ConfigStore is the official CLI tool for managing the RestInPieces framework's secure configuration store. It uses SQLite and age encryption to provide:

- Secure storage of framework secrets and configurations
- Versioning and rollback capabilities
- Scoped configurations (application-wide or per-component)
- JWT secret rotation
- OAuth2 provider management

## Features

- Secure storage using age encryption
- Configuration versioning and rollback
- Multiple configuration scopes
- TOML and JSON format support
- JWT secret management
- OAuth2 provider management

## Installation

```bash
go install github.com/caasmo/restinpieces/cmd/configstore
```

## Global Options

All commands require these global flags:
- `-age-key`: Path to age private key file
- `-db`: Path to SQLite database file

## Commands

### `set` - Set configuration values

Sets a configuration value at a given path. Creates new configuration versions.

```bash
configstore -age-key age.key -db config.db set -scope myapp -format toml -desc "Update API settings" api.url "https://api.example.com"
```

Options:
- `-scope`: Configuration scope (default: application)
- `-format`: Configuration format (toml/json)
- `-desc`: Description of this change

### `scopes` - List configuration scopes

Lists all unique configuration scopes in the database.

```bash
configstore -age-key age.key -db config.db scopes
```

### `list` - List configuration versions

Lists configuration versions, optionally filtered by scope. Shows generation number, scope, creation timestamp, format and description.

Example output:
```
Gen  Scope        Created At             Format  Description
---  ------------ ---------------------  ------  -----------
  0  application   2025-05-23T15:10:05Z   toml  Inserted from file: config.toml
  1  application   2025-05-21T19:38:04Z   toml  Inserted from file: config.toml
  2  application   2025-05-15T22:04:42Z   toml  Inserted from file: config.toml
  3  application   2025-04-27T15:28:24Z   toml  Updated field 'server.cert_data'
  4  application   2025-04-27T15:26:37Z   toml  Updated field 'server.cert_data'
  5  application   2025-04-27T15:25:54Z   toml  Updated field 'server.cert_data'
  6  application   2025-04-27T15:16:12Z   toml  Updated field 'scheduler.interval'
  7  application   2025-04-27T15:13:38Z   toml  Inserted from file: config.toml
  8  application   2025-04-27T15:04:41Z   toml  Updated field 'Scheduler.Interval'
```

Usage:
```bash
configstore -age-key age.key -db config.db list
configstore -age-key age.key -db config.db list myapp
```

### `paths` - List TOML paths

Lists all available TOML paths in the configuration, optionally filtered.

```bash
configstore -age-key age.key -db config.db paths -scope myapp
configstore -age-key age.key -db config.db paths -scope myapp "api.*"
```

### `dump` - Dump configuration

Outputs the latest configuration in plaintext.

```bash
configstore -age-key age.key -db config.db dump -scope myapp
```

### `diff` - Compare configurations

Shows differences between current and previous configuration versions.

```bash
configstore -age-key age.key -db config.db diff -scope myapp 1
```

### `rollback` - Restore any previous version

Rolls back to any previous configuration version by generation number. The generation number can be found using the `list` command.

```bash
# Rollback to generation 3 (any valid generation number can be used)
configstore -age-key age.key -db config.db rollback -scope myapp 3
```

### `save` - Save file contents

Saves the contents of a file to the configuration store.

```bash
configstore -age-key age.key -db config.db save -scope myapp config.toml
```

### `get` - Get configuration values

Retrieves configuration values by path.

```bash
configstore -age-key age.key -db config.db get -scope myapp "api.url"
```

### `init` - Initialize default config

Creates a new configuration with default values.

```bash
configstore -age-key age.key -db config.db init -scope myapp
```

### `rotate-jwt-secrets` - Rotate JWT secrets

Generates new random secrets for JWT tokens (application scope only).

```bash
configstore -age-key age.key -db config.db rotate-jwt-secrets
```

### `add-oauth2` - Add OAuth2 provider

Adds a new OAuth2 provider configuration skeleton.

```bash
configstore -age-key age.key -db config.db add-oauth2 gitlab
```

### `rm-oauth2` - Remove OAuth2 provider

Removes an OAuth2 provider configuration.

```bash
configstore -age-key age.key -db config.db rm-oauth2 gitlab
```

## Security Considerations

- Always protect the age private key file
- Database file should have restricted permissions
- Configuration may contain sensitive credentials
