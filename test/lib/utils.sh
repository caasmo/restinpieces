#!/bin/bash

# Global configuration
export SERVER_URL="http://localhost:8080"
export JWT_SECRET="test_secret_32_bytes_long_xxxxxx"
export TIMEOUT_SECONDS=5
export CURL_OPTS=("--silent" "--show-error" "--max-time" "$TIMEOUT_SECONDS")

# Configuration flags
VERBOSE=${VERBOSE:-true}  # Verbose by default

process_options() {
    while getopts "q" opt; do
        case $opt in
            q) VERBOSE=false ;;  # -q for quiet mode
            *) echo "Usage: $0 [-q]" >&2; exit 1 ;;
        esac
    done
    shift $((OPTIND-1))  # Remove processed options from arguments
}

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[1;36m'  # Light cyan (brighter blue)
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
    echo -e "${BLUE}🔍 [VALIDATION]${NC} ${BLUE}Environment Check${NC}"
    
    # Check required commands
    local missing=()
    log_info "Checking required commands...$"
    for cmd in curl jq go netstat lsof sqlite3; do
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
    
    echo -e "${GREEN}✓ Environment validation passed${NC}\n"
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

    log_debug "Curl command: ${curl_cmd[@]} $url"

    # Execute curl and capture both exit status and HTTP status code
    local curl_exit_status
    status_ref=$("${curl_cmd[@]}" "$url")
    curl_exit_status=$?

    # Check if curl itself failed (network error, timeout, etc.)
    if [ $curl_exit_status -ne 0 ]; then
        echo -e "${RED}[ERROR] Curl command failed with exit code $curl_exit_status${NC}" >&2
        status_ref=-1  # Set a special status code to indicate curl failure
    fi

    log_debug "Response status: $status_ref"
    [ -f "$response_file" ] && log_debug "Response body:\n$(cat "$response_file")"

    return $curl_exit_status  # Return the curl exit status
}

# Test counters
declare -g TESTS_RUN=0
declare -g TESTS_PASSED=0
declare -g TESTS_FAILED=0

# Logging functions
log_debug() {
      # Don't rely on the value when sourced, check current value each time
    if [[ "${VERBOSE:-true}" == "true" ]]; then
        echo -e "${YELLOW}[DEBUG] $*${NC}" >&2
    fi
}

log_info() {
    echo -e "${GREEN}[INFO] $*${NC}" >&2
}

log_error() {
    echo -e "${RED}[ERROR] $*${NC}" >&2
}

log_warning() {
    echo -e "${YELLOW}[WARNING] $*${NC}" >&2
}

# Start server with given database file
start_server() {
    local db_file=$1
    local port=${2:-8080}  # Default port 8080 if not specified

    # Validate input
    if [[ -z "$db_file" ]]; then
        log_error "Database file not provided"
        return 1
    fi

    # Check if port is already in use
    if netstat -tuln | grep -q ":$port "; then
        local existing_pid=$(lsof -ti:$port)
        if [[ -n "$existing_pid" ]]; then
            log_error "Port $port is already in use by PID $existing_pid"
            return 1
        else
            log_error "Port $port is already in use but couldn't identify the process"
            return 1
        fi
    fi

    #if [[ ! -f "$db_file" ]]; then
    #    log_warning "Database file '$db_file' doesn't exist. It will be created."
    #fi

    log_debug "Starting server with DB: $db_file"

    go run ./cmd/restinpieces/... -dbfile "$db_file" > /dev/null 2>&1 &
    log_debug "Waiting for server to initialize..."
    sleep 3 # Give server time to start


    # The $! variable captures the PID of the most recently executed background
    # process, which in this case is the go run command. However, go run is a
    # tool that compiles and then executes your Go program. The actual server
    # process is a child process of go run, not the go run process itself.

    # Find the actual PID bound to the port
    local pid=$(lsof -ti:$port)
    if [[ -z "$pid" ]]; then
        log_error "Failed to find process using port $port"
        return 1
    fi

    # Verify server is running
    if ! ps -p $pid > /dev/null; then
        log_error "Server failed to start"
        return 1
    fi

    log_info "Server started with PID: $pid"
    log_info "Server running at http://localhost:8080"

    # Return only the PID
    echo "$pid"
}

# Improved stop function that takes PID as argument
stop_server() {
    log_info "Stopping server"
    local pid=$1

    if [[ -z "$pid" ]]; then
        log_error "No PID provided to stop_server"
        return 1
    fi

    if ! ps -p $pid > /dev/null; then
        log_warning "No process found with PID $pid"
        return 0
    fi

    log_info "Stopping server with PID: $pid"
    kill $pid

    # Optionally wait to confirm process stopped
    sleep 1
    if ps -p $pid > /dev/null; then
        log_warning "Process $pid did not stop, attempting force kill"
        kill -9 $pid
    fi

    log_info "Server stopped"
    return 0
}

# Start a new test case
begin_test() {
    local test_name=$1
    echo -e "${BLUE}🚀 [TEST]${NC} ${BLUE}$test_name${NC}"
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

cleanup() {
    local db_file=$1
    
    if [[ -z "$db_file" ]]; then
        log_error "No database file provided for cleanup"
        return 1
    fi

    log_info "Cleaning up test artifacts"
    
    # Remove response files
    rm -f response*.txt
    
    # Remove test database file
    if [[ -f "$db_file" ]]; then
        log_debug "Removing test database: $db_file"
        rm -f "$db_file"
    else
        log_warning "Test database file not found: $db_file"
    fi
    
    return 0
}

# Setup a temporary test database with schema
setup_test_db() {
    local db_file=$(mktemp -t testdb_XXXXXX.db)
    log_debug "Creating test database: $db_file"
    
    # Load schema from migrations/users.sql
    if ! sqlite3 "$db_file" < migrations/users.sql; then
        log_error "Failed to initialize test database schema"
        rm -f "$db_file"
        return 1
    fi
    
    echo "$db_file"  # Return generated filename
    return 0
}


print_test_summary() {
    echo -e "📊${BLUE} [SUMMARY]${NC} ${BLUE}Test Results${NC}"
    echo -e "Tests Run:   $TESTS_RUN"
    echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
    
    if [ "$TESTS_FAILED" -gt 0 ]; then
        echo -e "${RED}❌ Some tests failed!${NC}"
        return 1
    else
        echo -e "${GREEN}✅ All tests passed!${NC}"
        return 0
    fi
}
