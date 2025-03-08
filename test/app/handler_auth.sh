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
    
    local response_file="response_$$.txt"
    local status
    
    http_request POST "/auth-refresh" status "$response_file" "" \
        "Authorization: Bearer $token"
        
    if assert_status 200 "$status"; then
        assert_json_contains "access_token" "$response_file"
    fi
    
    [ $? -eq 0 ] && log_success || true
}

test_invalid_token() {
    log_test_start "Invalid token"
    
    local response_file="response_$$.txt"
    local status
    
    http_request POST "/auth-refresh" status "$response_file" "" \
        "Authorization: Bearer invalid.token.here"
        
    assert_status 401 "$status "Expected 401 for invalid token"
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
