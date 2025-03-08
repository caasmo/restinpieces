#!/bin/bash
set -eo pipefail

# Source utilities
TEST_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "$TEST_ROOT/lib/utils.sh"

test_valid_token_refresh() {
    log_test_start "Valid token refresh"
    
    local token
    if ! token=$(jwt "$JWT_SECRET" "testuser123" "+5 minutes"); then
        log_failure "Token generation failed"
        return 1
    fi
    
    if $VERBOSE; then
        echo -e "${YELLOW}[DEBUG] Generated JWT token: $token${NC}"
        
        # Validate token format
        if [[ $(grep -o '\.' <<< "$token" | wc -l) -ne 2 ]]; then
            echo -e "${RED}[ERROR] Invalid JWT format - expected 3 parts${NC}"
            return 1
        fi
        
        # Verify secret length
        if [ ${#JWT_SECRET} -lt 32 ]; then
            echo -e "${RED}[ERROR] JWT_SECRET too short - needs 32 bytes${NC}"
            return 1
        fi
        
        # Decode token components for inspection
        IFS=. read header payload _ <<< "$token"
        echo -e "${YELLOW}[DEBUG] Header: $(base64 -d <<< "$header")${NC}"
        echo -e "${YELLOW}[DEBUG] Payload: $(base64 -d <<< "$payload")${NC}"
    fi
    
    local response_file="response_$$.txt"
    local status
    
    http_request POST "/auth-refresh" status "$response_file" "" \
        "Authorization: Bearer $token"
        
    if assert_status 200 "$status"; then
        assert_json_contains "access_token" "$response_file"
    else
        echo -e "${RED}Response status: $status${NC}"
        [ -f "$response_file" ] && echo -e "${YELLOW}Response body:\n$(cat "$response_file")${NC}"
    fi
    
    if [ $? -eq 0 ]; then
        log_success
    else
        log_failure "Token refresh failed"
        return 1
    fi
}

test_invalid_token() {
    log_test_start "Invalid token"
    
    local response_file="response_$$.txt"
    local status
    
    http_request POST "/auth-refresh" status "$response_file" "" \
        "Authorization: Bearer invalid.token.here"
        
    assert_status 401 "$status" "Expected 401 for invalid token"
    [ $? -eq 0 ] && log_success || true
}

test_missing_auth_header() {
    log_test_start "Missing authorization header"
    
    local response_file="response_$$.txt"
    local status
    
    http_request POST "/auth-refresh" status "$response_file"
    
    assert_status 400 "$status" "Expected 400 for missing auth header"
    [ $? -eq 0 ] && log_success || true
}

test_valid_registration() {
    log_test_start "Valid user registration"
    
    local response_file="response_$$.txt"
    local status
    
    http_request POST "/register" status "$response_file" \
        '{"identity":"new@test.com","password":"pass123","password_confirm":"pass123"}' \
        "Content-Type: application/json"
        
    if assert_status 200 "$status"; then
        assert_json_contains "token" "$response_file" && \
        assert_json_contains "record.id" "$response_file"
    fi
    
    [ $? -eq 0 ] && log_success || true
}

main() {
    # Parse command line arguments
    while getopts "v" opt; do
        case $opt in
            v) VERBOSE=true ;;
            *) echo "Usage: $0 [-v]" >&2; exit 1 ;;
        esac
    done

    validate_environment
    
    # Run tests
    test_valid_token_refresh
    test_invalid_token
    test_missing_auth_header
    test_valid_registration
    
    cleanup
    print_test_summary
}

main
