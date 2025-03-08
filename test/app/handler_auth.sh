#!/bin/bash
#set -eo pipefail

# Source utilities
TEST_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "$TEST_ROOT/lib/utils.sh"

test_valid_token_refresh() {
    log_test_start "/auth-refresh: Valid token refresh"

    # Generate and display token first
    local token=$(jwt "$JWT_SECRET" "testuser123" "+5 minutes")
    
    if $VERBOSE; then
        echo -e "${YELLOW}[DEBUG] Raw JWT token: $token${NC}"
        
    # Basic token validation checks
    if [[ -z "$token" ]]; then
        log_failure "Empty token generated"
        return 1
    fi
    
    if [[ $(grep -o '\.' <<< "$token" | wc -l) -ne 2 ]]; then
        log_failure "Invalid JWT format - got ${RED}$(wc -l <<< "$token") parts${NC}, expected 3"
        return 1
    fi
    
    if [ ${#JWT_SECRET} -ne 32 ]; then
        log_failure "JWT_SECRET must be 32 bytes, got ${#JWT_SECRET}"
        return 1
    fi
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
    log_test_start "/auth-refresh: Invalid token"
    
    local response_file="response_$$.txt"
    local status
    
    http_request POST "/auth-refresh" status "$response_file" "" \
        "Authorization: Bearer invalid.token.here"
        
    assert_status 401 "$status" "Expected 401 for invalid token"
    [ $? -eq 0 ] && log_success || true
}

test_missing_auth_header() {
    log_test_start "/auth-refresh: Missing authorization header"
    
    local response_file="response_$$.txt"
    local status
    
    http_request POST "/auth-refresh" status "$response_file"
    
    assert_status 401 "$status" "Expected 401 for missing auth header. From middleware"
    [ $? -eq 0 ] && log_success || true
}

test_valid_registration() {
    log_test_start "/register: Valid user registration"
    
    local response_file="response_$$.txt"
    local status
    
    http_request POST "/register" status "$response_file" \
        '{"identity":"new@test.com","password":"pass1234","password_confirm":"pass1234"}' \
        "Content-Type: application/json"
        
    if assert_status 200 "$status"; then
        assert_json_contains "token" "$response_file" && \
        assert_json_contains "record.id" "$response_file"
    fi
    
    [ $? -eq 0 ] && log_success || true
}

main() {
    # Parse command line arguments
    while getopts "q" opt; do
        case $opt in
            q) VERBOSE=false ;;  # -q for quiet mode
            *) echo "Usage: $0 [-q]" >&2; exit 1 ;;
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
