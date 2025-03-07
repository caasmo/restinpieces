#!/bin/bash

# Global configuration
export SERVER_URL="http://localhost:8080"
export JWT_SECRET="test_secret_32_bytes_long_xxxxxx"
export TIMEOUT_SECONDS=5
export CURL_OPTS=("--silent" "--show-error" "--max-time" "$TIMEOUT_SECONDS")

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counters
declare -g TESTS_RUN=0
declare -g TESTS_PASSED=0
declare -g TESTS_FAILED=0

jwt() {
    # Usage: generate_jwt <secret> <user_id> [expiry_time]
    local secret=$1
    local user_id=$2
    local expiry=${3:-"+5 minutes"}

    local header=$(printf '{"alg":"HS256","typ":"JWT"}' | base64 | tr -d '=\n' | tr '/+' '_-')
    local exp=$(date -d "$expiry" +%s)
    local payload=$(printf '{"user_id":"%s","exp":%d}' "$user_id" "$exp" | base64 | tr -d '=\n' | tr '/+' '_-')

    local signature=$(printf "%s.%s" "$header" "$payload" |
                      openssl dgst -sha256 -hmac "$secret" -binary |
                      base64 | tr -d '=\n' | tr '/+' '_-')

    printf "%s.%s.%s\n" "$header" "$payload" "$signature"
}

validate_environment() {
    # Check required commands
    local missing=()
    for cmd in curl jq go; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done
    
    if [ ${#missing[@]} -gt 0 ]; then
        echo -e "${RED}Missing required commands: ${missing[*]}${NC}"
        exit 1
    fi
    
    # Check server connectivity
    if ! curl "${CURL_OPTS[@]}" "$SERVER_URL" &>/dev/null; then
        echo -e "${RED}Could not connect to server at $SERVER_URL${NC}"
        exit 1
    fi
}

http_request() {
    # Generic HTTP request helper
    local method=$1
    local path=$2
    local status_var=$3
    local response_file=$4
    local data=$5
    local headers=("${@:6}")
    
    local url="${SERVER_URL}${path}"
    local curl_cmd=("curl" "${CURL_OPTS[@]}" "-X" "$method" "-o" "$response_file" "-w" "%{http_code}")
    
    for header in "${headers[@]}"; do
        curl_cmd+=("-H" "$header")
    done
    
    if [ -n "$data" ]; then
        curl_cmd+=("-d" "$data")
    fi
    
    local status_code
    status_code=$("${curl_cmd[@]}" "$url")
    eval "$status_var=$status_code"
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

assert_status() {
    local expected=$1
    local actual=$2
    local message=${3:-"Expected status $expected, got $actual"}
    
    if [ "$actual" -ne "$expected" ]; then
        log_failure "$message"
        return 1
    fi
    return 0
}

assert_json_contains() {
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
    # Add other cleanup tasks here
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
