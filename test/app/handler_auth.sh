#!/bin/bash
#set -eo pipefail

# Process command line options
process_options "$@"

# Source utilities
TEST_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "$TEST_ROOT/lib/utils.sh"

test_auth_refresh_valid() {
    begin_test "/auth-refresh: Valid token refresh"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    # Generate token
    local token=$(jwt "$JWT_SECRET" "testuser123" "+5 minutes")
    log_debug "Generated JWT token: $token"

    # Basic token validation
    if [[ -z "$token" ]]; then
        end_test 1 "Empty token generated"
        return 1
    fi
    
    if [[ $(grep -o '\.' <<< "$token" | wc -l) -ne 2 ]]; then
        end_test 1 "Invalid JWT format - got $(wc -l <<< "$token") parts, expected 3"
        return 1
    fi
    
    if [ ${#JWT_SECRET} -ne 32 ]; then
        end_test 1 "JWT_SECRET must be 32 bytes, got ${#JWT_SECRET}"
        return 1
    fi

    http_request POST "/auth-refresh" status "$response_file" "" \
        "Authorization: Bearer $token"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 200 "$status" "Expected 200 for valid token refresh" || test_result=1
    assert_json_contains "access_token" "$response_file" "Response missing access_token" || test_result=1

    if [ $test_result -ne 0 ]; then
        log_debug "Response status: $status"
        [ -f "$response_file" ] && log_debug "Response body:\n$(cat "$response_file")"
    fi

    end_test $test_result "One or more assertions failed"
    return $test_result
}

test_auth_refresh_valid_after_register() {
    begin_test "/auth-refresh: Valid token refresh after registration"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    # Register new user
    http_request POST "/register" status "$response_file" \
        '{"identity":"auth-refresh_valid@test.com","password":"password-auth-refresh","password_confirm":"password-auth-refresh"}' \
        "Content-Type: application/json"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Failed to register test user"
        return 1
    fi

    # Extract token from registration response
    local token=$(jq -r '.token' "$response_file")
    if [[ -z "$token" ]]; then
        end_test 1 "Failed to extract token from registration response"
        return 1
    fi

    # Test token refresh
    http_request POST "/auth-refresh" status "$response_file" "" \
        "Authorization: Bearer $token"
    request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 200 "$status" "Expected 200 for valid token refresh" || test_result=1
    assert_json_contains "access_token" "$response_file" "Response missing access_token" || test_result=1

    if [ $test_result -ne 0 ]; then
        log_debug "Response status: $status"
        [ -f "$response_file" ] && log_debug "Response body:\n$(cat "$response_file")"
    fi

    end_test $test_result "One or more assertions failed"
    return $test_result
}

test_auth_refresh_invalid_token() {
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
    assert_json_contains "error" "$response_file" "Response missing error details" || test_result=1
    end_test $test_result "One or more assertions failed"
    return $test_result
}


test_auth_refresh_missing_header() {
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
    assert_json_contains "error" "$response_file" "Response missing error details" || test_result=1

    end_test $test_result "One or more assertions failed"
    return $test_result
}


test_register_valid() {
    begin_test "/register: Valid user registration"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    http_request POST "/register" status "$response_file" \
        '{"identity":"register_valid@test.com","password":"password-register","password_confirm":"password-register"}' \
        "Content-Type: application/json"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 200 "$status" "Expected 200 for valid registration" || test_result=1
    assert_json_contains "token" "$response_file" "Response missing token" || test_result=1
    assert_json_contains "record.id" "$response_file" "Response missing record ID" || test_result=1

    end_test $test_result "One or more assertions failed"
    return $test_result
}

test_register_duplicate_email() {
    begin_test "/register: Invalid registration (existing email)"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    # First registration should succeed
    http_request POST "/register" status "$response_file" \
        '{"identity":"register_duplicate@test.com","password":"password-register","password_confirm":"password-register"}' \
        "Content-Type: application/json"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    # Verify first registration succeeded
    assert_status 200 "$status" "Expected 200 for first registration" || test_result=1
    assert_json_contains "token" "$response_file" "Response missing token" || test_result=1
    assert_json_contains "record.id" "$response_file" "Response missing record ID" || test_result=1

    # Second registration with same email should fail
    http_request POST "/register" status "$response_file" \
        '{"identity":"register_duplicate@test.com","password":"password-register","password_confirm":"password-register"}' \
        "Content-Type: application/json"
    request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    # Verify second registration failed
    assert_status 409 "$status" "Expected 409 for duplicate registration" || test_result=1
    assert_json_contains "error" "$response_file" "Response missing error details" || test_result=1

    end_test $test_result "One or more assertions failed"
    return $test_result
}

test_auth_with_password_valid_credentials() {
    begin_test "/auth-with-password: Valid credentials"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    # First register a test user
    http_request POST "/register" status "$response_file" \
        '{"identity":"auth-with-password_valid@test.com","password":"password-register","password_confirm":"password-register"}' \
        "Content-Type: application/json"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Failed to register test user"
        return 1
    fi

    # Test authentication with valid credentials
    http_request POST "/auth-with-password" status "$response_file" \
        '{"identity":"auth-with-password_valid@test.com","password":"password-register"}' \
        "Content-Type: application/json"
    request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 200 "$status" "Expected 200 for valid credentials" || test_result=1
    assert_json_contains "token" "$response_file" "Response missing token" || test_result=1
    assert_json_contains "record" "$response_file" "Response missing user record" || test_result=1

    end_test $test_result "One or more assertions failed"
    return $test_result
}

test_auth_with_password_invalid_credentials() {
    begin_test "/auth-with-password: Invalid credentials"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    # Test authentication with invalid password
    http_request POST "/auth-with-password" status "$response_file" \
        '{"identity":"auth-with-password_valid@test.com","password":"password-auth-with-password-wrong"}' \
        "Content-Type: application/json"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 401 "$status" "Expected 401 for invalid credentials" || test_result=1
    assert_json_contains "error" "$response_file" "Response missing error details" || test_result=1

    end_test $test_result "One or more assertions failed"
    return $test_result
}

test_auth_with_password_missing_fields() {
    begin_test "/auth-with-password: Missing required fields"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    # Test authentication with missing password
    http_request POST "/auth-with-password" status "$response_file" \
        '{"identity":"auth-with-password_missing@test.com"}' \
        "Content-Type: application/json"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 400 "$status" "Expected 400 for missing fields" || test_result=1
    assert_json_contains "error" "$response_file" "Response missing error details" || test_result=1

    end_test $test_result "One or more assertions failed"
    return $test_result
}

main() {
    validate_environment
    
    # Setup test database for all tests in this file
    db_file=$(setup_test_db)
    log_info "Using test database: $db_file"

    # Start server with test database
    server_pid=$(start_server "$db_file")
    exit_code=$?

    if [[ $exit_code -ne 0 ]]; then
        log_error "Failed to start server"
        exit $exit_code
    fi

    log_info "Server started successfully with PID: $server_pid"

    
    # Run tests
    
    # /auth-refresh endpoint tests
    test_auth_refresh_valid
    test_auth_refresh_valid_after_register
    test_auth_refresh_invalid_token
    test_auth_refresh_missing_header
    
    # /register endpoint tests
    test_register_valid
    test_register_duplicate_email
    
    # /auth-with-password endpoint tests
    test_auth_with_password_valid_credentials
    test_auth_with_password_invalid_credentials
    test_auth_with_password_missing_fields
    
    print_test_summary
    stop_server "$server_pid"
    cleanup "$db_file"
}

main
