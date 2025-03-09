#!/bin/bash

# Global configuration
export SERVER_URL="http://localhost:8080"
export JWT_SECRET="test_secret_32_bytes_long_xxxxxx"
export TIMEOUT_SECONDS=5
export CURL_OPTS=("--silent" "--show-error" "--max-time" "$TIMEOUT_SECONDS")

# Configuration flags
declare -g VERBOSE=${VERBOSE:-true}  # Verbose by default

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

jwt() {
     # Usage: generate_jwt <secret> <user_id> [expiry_time]
     # Example: generate_jwt "mysupersecret" "testuser123" "+5 minutes"
     local secret=$1
     local user_id=$2
     local expiry=${3:-"+5 minutes"}  # Default to 5 minutes

     # Create header and payload
     local header=$(printf '{"alg":"HS256","typ":"JWT"}' | base64 | tr -d '=\n' | tr '/+' '_-')
     local exp=$(date -d "$expiry" +%s)
     local payload=$(printf '{"user_id":"%s","exp":%d}' "$user_id" "$exp" | base64 | tr -d '=\n' | tr '/+' '_-')

     # Create signature
     local signature=$(printf "%s.%s" "$header" "$payload" |
                       openssl dgst -sha256 -hmac "$secret" -binary |
                       base64 | tr -d '=\n' | tr '/+' '_-')

     # Combine to form JWT
     printf "%s.%s.%s\n" "$header" "$payload" "$signature"
 }

validate_environment() {
    echo -e "${YELLOW}=== Environment Validation ===${NC}"
    
    # Verify test server is running
    if ! curl "${CURL_OPTS[@]}" "$SERVER_URL" &>/dev/null; then
        echo -e "${RED}Error: Test server is not running at ${SERVER_URL}${NC}"
        exit 1
    fi
    
    # Check required commands
    local missing=()
    echo -e "${YELLOW}Checking required commands...${NC}"
    for cmd in curl jq go; do
        if command -v "$cmd" &>/dev/null; then
            echo -e "  ${GREEN}✓${NC} $cmd found"
        else
            echo -e "  ${RED}✗${NC} $cmd missing"
            missing+=("$cmd")
        fi
    done
    
    if [ ${#missing[@]} -gt 0 ]; then
        echo -e "${RED}Error: Missing required commands: ${missing[*]}${NC}"
        exit 1
    fi
    
    # Check server connectivity
    echo -e "${YELLOW}Checking server connection...${NC}"
    if curl "${CURL_OPTS[@]}" "$SERVER_URL" &>/dev/null; then
        echo -e "  ${GREEN}✓${NC} Successfully connected to ${SERVER_URL}"
    else
        echo -e "  ${RED}✗${NC} Could not connect to server at ${SERVER_URL}"
        echo -e "${RED}Error: Verify the server is running and accessible${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Environment validation passed${NC}\n"
}

http_request() {
    # Generic HTTP request helper
    local method=$1
    local path=$2
    local -n status_ref=$3  # Use nameref
    local response_file=$4
    local data=$5
    local headers=("${@:6}")

    local url="${SERVER_URL}${path}"
    local curl_cmd=("curl" "${CURL_OPTS[@]}" "-X" "$method" "-o" "$response_file" "-w" "%{http_code}")

    for header in "${headers[@]}"; do
        curl_cmd+=("-H" "$header")
    done

    if [ -n "$data" ]; then
        curl_cmd+=("--data-binary" "$data")
    fi

    if $VERBOSE; then
        echo -e "\n${YELLOW}[DEBUG] Curl command:${NC}"
        echo "${curl_cmd[@]} $url"
    fi

    # Execute curl and capture both exit status and HTTP status code
    local curl_exit_status
    status_ref=$("${curl_cmd[@]}" "$url")
    curl_exit_status=$?

    # Check if curl itself failed (network error, timeout, etc.)
    if [ $curl_exit_status -ne 0 ]; then
        echo -e "${RED}[ERROR] Curl command failed with exit code $curl_exit_status${NC}" >&2
        status_ref=-1  # Set a special status code to indicate curl failure
    fi

    if $VERBOSE; then
        echo -e "${YELLOW}[DEBUG] Response status: $status_ref${NC}"
        [ -f "$response_file" ] && echo -e "${YELLOW}[DEBUG] Response body:\n$(cat "$response_file")${NC}"
    fi

    return $curl_exit_status  # Return the curl exit status
}

# Test counters
declare -g TESTS_RUN=0
declare -g TESTS_PASSED=0
declare -g TESTS_FAILED=0

# Start a new test case
begin_test() {
    local test_name=$1
    echo -e "${YELLOW}=== TEST: $test_name ===${NC}"
    ((TESTS_RUN++))
    return 0
}

# End a test case with success or failure
end_test() {
    local result=$1
    local message=$2

    if [ $result -eq 0 ]; then
        echo -e "${GREEN}PASS${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}FAIL: $message${NC}"
        ((TESTS_FAILED++))
    fi

    return $result
}

# Assertion functions that don't modify counters
assert_status() {
    local expected=$1
    local actual=$2
    local message=${3:-"Expected status $expected, got $actual"}

    if [ "$actual" -ne "$expected" ]; then
        echo -e "  ${RED}× Status assertion failed: $message${NC}"
        return 1
    fi
    echo -e "  ${GREEN}✓ Status assertion passed${NC}"
    return 0
}

assert_json_contains() {
    local key=$1
    local file=$2

    if ! jq -e ".$key" "$file" >/dev/null; then
        echo -e "  ${RED}× JSON assertion failed: Missing $key in response${NC}"
        return 1
    fi
    echo -e "  ${GREEN}✓ JSON assertion passed${NC}"
    return 0
}



















log_test_start() {
    local test_name=$1
    echo -e "${YELLOW}=== TEST: $test_name ===${NC}"
    ((TESTS_RUN++))
}

log_success() {
    echo -e "${GREEN}PASS${NC}"
    ((TESTS_PASSED++))
}

log_failure() {
    local message=$1
    echo -e "${RED}FAIL: $message${NC}"
    ((TESTS_FAILED++))
}

aassert_status() {
    local expected=$1
    local actual=$2
    local message=${3:-"Expected status $expected, got $actual"}
    
    if [ "$actual" -ne "$expected" ]; then
        log_failure "$message"
        return 1
    fi
    return 0
}

aassert_json_contains() {
    local key=$1
    local file=$2
    if ! jq -e ".$key" "$file" >/dev/null; then
        log_failure "Missing $key in response"
        return 1
    fi
    return 0
}

cleanup() {
    rm -f response*.txt
    cleanup_test_db
    
    # Kill server if running
    if ps -p $server_pid > /dev/null; then
        echo -e "${YELLOW}[DEBUG] Stopping test server (PID: $server_pid)${NC}"
        kill -TERM $server_pid
        wait $server_pid 2>/dev/null
    fi
}

# Setup a temporary test database with schema
setup_test_db() {
    local db_file=$(mktemp -t testdb_XXXXXX.db)
    echo "$db_file"  # Return generated filename
    
    # Load schema from migrations/users.sql
    sqlite3 "$db_file" < migrations/users.sql
}

# Start server with given database file
start_server() {
    local db_file=$1
    if $VERBOSE; then
        echo -e "${YELLOW}[DEBUG] Starting server with DB: $db_file${NC}"
    fi
    
    go run ./cmd/restinpieces/... -dbfile "$db_file" > /dev/null 2>&1 &
    server_pid=$!
    sleep 3 # Give server time to start
    
    if $VERBOSE; then
        echo -e "${YELLOW}[DEBUG] Server started with PID: $server_pid${NC}"
    fi
    
    echo "$server_pid"
}

# Cleanup database files
cleanup_test_db() {
    rm -f testdb_*.db
}

print_test_summary() {
    echo -e "\n${YELLOW}=== Test Summary ===${NC}"
    echo -e "Tests Run:   $TESTS_RUN"
    echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
    
    if [ "$TESTS_FAILED" -gt 0 ]; then
        echo -e "${RED}Some tests failed!${NC}"
        exit 1
    else
        echo -e "${GREEN}All tests passed!${NC}"
    fi
}
