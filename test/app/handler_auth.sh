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

    begin_test "/auth-refresh: Invalid token"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    http_request POST "/auth-refresh" status "$response_file" "" \
        "Authorization: Bearer invalid.token.here"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 401 "$status" "Expected 401 for invalid token" || test_result=1
    end_test $test_result "One or more assertions failed"
    return $test_result
}


test_missing_auth_header() {
    begin_test "/auth-refresh: Missing authorization header"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    http_request POST "/auth-refresh" status "$response_file"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 401 "$status" "Expected 401 for missing auth header" || test_result=1
    assert_json_contains "error" "$response_file" || test_result=1

    end_test $test_result "One or more assertions failed"
    return $test_result
}


test_valid_registration() {
    begin_test "/register: Valid user registration"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    http_request POST "/register" status "$response_file" \
        '{"identity":"new@test.com","password":"pass1234","password_confirm":"pass1234"}' \
        "Content-Type: application/json"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 200 "$status" "Expected 200 for valid registration" || test_result=1
    assert_json_contains "token" "$response_file" || test_result=1
    assert_json_contains "record.id" "$response_file" || test_result=1

    end_test $test_result "One or more assertions failed"
    return $test_result
}

test_invalid_registration() {
    log_test_start "/register: Invalid registration (existing email)"
    
    local response_file="response_$$.txt"
    local status
    
    # First registration
    http_request POST "/register" status "$response_file" \
        '{"identity":"existing@test.com","password":"pass1234","password_confirm":"pass1234"}' \
        "Content-Type: application/json"
        
    # Second registration with same email
    http_request POST "/register" status "$response_file" \
        '{"identity":"existing@test.com","password":"pass1234","password_confirm":"pass1234"}' \
        "Content-Type: application/json"
        
    assert_status 409 "$status" "Expected 409 for duplicate registration"
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
    
    # Setup test database for all tests in this file
    db_file=$(setup_test_db)
    if $VERBOSE; then
        echo -e "${YELLOW}[DEBUG] Using test database: $db_file${NC}"
    fi
    
    # Run tests
    #test_valid_token_refresh
    test_invalid_token
    test_missing_auth_header
    test_valid_registration
    #test_invalid_registration
    
    cleanup
    print_test_summary
}

main
