# ConfigStore - Secure Configuration Management

ConfigStore is a CLI tool for securely managing application configurations using SQLite and age encryption. It provides versioning, scoping, and secure storage of sensitive configuration data.

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

Lists configuration versions, optionally filtered by scope.

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

### `rollback` - Restore previous version

Rolls back to a previous configuration version.

```bash
configstore -age-key age.key -db config.db rollback -scope myapp 1
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

### `renew-jwt-secrets` - Rotate JWT secrets

Generates new random secrets for JWT tokens (application scope only).

```bash
configstore -age-key age.key -db config.db renew-jwt-secrets
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
