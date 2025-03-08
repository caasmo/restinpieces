#!/bin/bash

# Global configuration
export SERVER_URL="http://localhost:8080"
export JWT_SECRET="test_secret_32_bytes_long_xxxxxx"
export TIMEOUT_SECONDS=5
export CURL_OPTS=("--silent" "--show-error" "--max-time" "$TIMEOUT_SECONDS")

# Configuration flags
declare -g VERBOSE=${VERBOSE:-false}

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
    local expiry=${3:-"5 min"}
    
    # Generate expiration timestamp
    local exp=$(date -d "$expiry" +%s)
    
    # Create header and payload with proper base64 encoding
    local header=$(echo -n '{"alg":"HS256","typ":"JWT"}' | base64 -w 0 | tr '/+' '_-')
    local payload=$(echo -n "{\"user_id\":\"$user_id\",\"exp\":$exp}" | base64 -w 0 | tr '/+' '_-')
    
    # Create signature
    local sig_input="${header}.${payload}"
    local signature=$(echo -n "$sig_input" | openssl dgst -sha256 -hmac "$secret" -binary | base64 -w 0 | tr '/+' '_-')
    
    printf "%s.%s.%s\n" "$header" "$payload" "$signature"
}

validate_environment() {
    echo -e "${YELLOW}=== Environment Validation ===${NC}"
    
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
    
    if $VERBOSE; then
        echo -e "\n${YELLOW}[DEBUG] Curl command:${NC}"
        echo "${curl_cmd[@]} $url"
    fi
    
    local status_code
    status_code=$("${curl_cmd[@]}" "$url")
    eval "$status_var=$status_code"
    
    if $VERBOSE; then
        echo -e "${YELLOW}[DEBUG] Response status: $status_code${NC}"
        [ -f "$response_file" ] && echo -e "${YELLOW}[DEBUG] Response body:\n$(cat "$response_file")${NC}"
    fi
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
