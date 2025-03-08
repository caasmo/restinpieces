#!/bin/bash
set -o errexit
set -o pipefail
set -o nounset

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SERVER_URL="http://localhost:8080"
JWT_SECRET="test_secret_32_bytes_long_xxxxxx" # Must match app config
TIMEOUT_SECONDS=5
CURL_OPTS=("--silent" "--show-error" "--max-time" "$TIMEOUT_SECONDS")

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

test_valid_token_refresh() {
    ((TESTS_RUN++))
    echo -e "${YELLOW}=== TEST: Valid token refresh ===${NC}"
    
    # Generate valid test token
    local token
    if ! token=$(jwt encode --secret "$JWT_SECRET" --claim user_id=testuser123 --exp +5m); then
        echo -e "${RED}FAIL: Token generation failed${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Make refresh request
    local response
    if ! response=$(curl "${CURL_OPTS[@]}" -o response.txt -w "%{http_code}" \
        -X POST "$SERVER_URL/auth-refresh" \
        -H "Authorization: Bearer $token"); then
        echo -e "${RED}FAIL: Request failed${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
        
    # Validate response
    if [ "$response" -ne 200 ]; then
        echo -e "${RED}FAIL: Expected 200, got $response${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
    
    if ! jq -e '.access_token' response.txt >/dev/null; then
        echo -e "${RED}FAIL: Missing access_token in response${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
    
    echo -e "${GREEN}PASS${NC}"
    ((TESTS_PASSED++))
}

test_invalid_token() {
    ((TESTS_RUN++))
    echo -e "${YELLOW}=== TEST: Invalid token ===${NC}"
    
    response=$(curl "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" \
        -X POST "$SERVER_URL/auth-refresh" \
        -H "Authorization: Bearer invalid.token.here")
        
    if [ "$response" -eq 401 ]; then
        echo -e "${GREEN}PASS${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}FAIL: Expected 401, got $response${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
}

test_missing_auth_header() {
    ((TESTS_RUN++))
    echo -e "${YELLOW}=== TEST: Missing authorization header ===${NC}"
    
    response=$(curl "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" \
        -X POST "$SERVER_URL/auth-refresh")
        
    if [ "$response" -eq 400 ]; then
        echo -e "${GREEN}PASS${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}FAIL: Expected 400, got $response${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
}

test_valid_registration() {
    ((TESTS_RUN++))
    echo -e "${YELLOW}=== TEST: Valid user registration ===${NC}"
    
    response=$(curl "${CURL_OPTS[@]}" -o response.txt -w "%{http_code}" \
        -X POST "$SERVER_URL/register" \
        -H "Content-Type: application/json" \
        -d '{
            "identity": "newuser@test.com",
            "password": "securePass123!",
            "password_confirm": "securePass123!"
        }')
        
    if [ "$response" -ne 200 ]; then
        echo -e "${RED}FAIL: Expected 200, got $response${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
    
    if ! jq -e '.token' response.txt >/dev/null; then
        echo -e "${RED}FAIL: Missing token in response${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
    
    if ! jq -e '.record.id' response.txt >/dev/null; then
        echo -e "${RED}FAIL: Missing user record in response${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
    
    echo -e "${GREEN}PASS${NC}"
    ((TESTS_PASSED++))
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

cleanup_test_data() {
    echo -e "${YELLOW}Cleaning up test data...${NC}"
    # Add cleanup commands here as needed
}

main() {
    validate_environment
    
    # Run tests
    test_valid_token_refresh
    test_invalid_token
    test_missing_auth_header
    test_valid_registration
    
    # Cleanup
    cleanup_test_data
    rm -f response.txt
    
    # Print summary
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

# Run main function
main
