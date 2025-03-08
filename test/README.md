# Test Suite Documentation

## Overview
The test suite contains two types of tests:
1. **Go unit tests** - Traditional Go test files (`*_test.go`)
2. **Integration test scripts** - Bash-based tests using curl (`test/app/*.sh`)

## Test Types

### Go Unit Tests
- Location: `app/`, `db/`, `crypto/` packages
- Tests: Handler logic, middleware, database operations, crypto functions
- Run all: 
  ```bash
  go test -v ./...
  ```
- Run specific package:
  ```bash
  go test -v ./app/...
  ```

### Integration Tests
- Location: `test/app/`
- Tests: API endpoints using real HTTP requests
- Dependencies: curl, jq, openssl, coreutils
- Example test:
  ```bash
  ./test/app/handler_auth.sh
  ```

## Running Tests

1. **Start the server** first in development mode:
```bash
make run-dev
```

2. **Run all Go tests**:
```bash
go test -v ./...
```

3. **Run integration tests**:
```bash
# Make scripts executable
chmod +x test/app/*.sh

# Run specific test script
./test/app/handler_auth.sh

# Run all integration tests
find test/app -name '*.sh' -exec {} \;
```

## Environment Setup
Tests require these environment variables:
```bash
export SERVER_URL="http://localhost:8080"
export JWT_SECRET="test_secret_32_bytes_long_xxxxxx" # Must match app config
```

## Script Structure
Integration tests follow this pattern:
1. Source utilities from `test/lib/utils.sh`
2. Define test functions with `log_test_start` and assertions
3. Use `http_request` helper for API calls
4. Validate responses with `assert_status` and `assert_json_contains`
5. Clean up temporary files

## Adding New Tests
1. For Go tests - create `*_test.go` files in the package directory
2. For integration tests - create new `.sh` files in `test/app/`
3. Put common helpers in `test/lib/utils.sh`
