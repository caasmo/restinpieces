#!/bin/bash
#set -eo pipefail

# Process command line options
process_options "$@"

# The endpoint to test - should be one that requires authorization
PROTECTED_ENDPOINT="/"

# Source utilities
TEST_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "$TEST_ROOT/lib/utils.sh"

test_middleware_auth_valid() {
    begin_test "JWT Middleware: Valid token"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    # Generate valid token
    local token=$(jwt "$JWT_SECRET" "middleware_valid" "+5 minutes")
    
    http_request GET "$PROTECTED_ENDPOINT" status "$response_file" "" \
        "Authorization: Bearer $token"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 200 "$status" "Expected 200 for valid token" || test_result=1
    end_test $test_result "Validation failed"
    return $test_result
}

test_middleware_auth_missing_header() {
    begin_test "JWT Middleware: Missing authorization header"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    http_request GET "$PROTECTED_ENDPOINT" status "$response_file"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 401 "$status" "Expected 401 for missing header" || test_result=1
    assert_json_contains "error" "$response_file" "Response missing error details" || test_result=1
    end_test $test_result "Validation failed"
    return $test_result
}

test_middleware_auth_invalid_format() {
    begin_test "JWT Middleware: Invalid token format"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    http_request GET "$PROTECTED_ENDPOINT" status "$response_file" "" \
        "Authorization: Bearer invalid.token.format"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 401 "$status" "Expected 401 for invalid token format" || test_result=1
    assert_json_contains "error" "$response_file" "Response missing error details" || test_result=1
    end_test $test_result "Validation failed"
    return $test_result
}

test_middleware_auth_expired_token() {
    begin_test "JWT Middleware: Expired token"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    # Generate expired token
    local token=$(jwt "$JWT_SECRET" "middleware_expired" "-1 minute")
    
    http_request GET "$PROTECTED_ENDPOINT" status "$response_file" "" \
        "Authorization: Bearer $token"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 401 "$status" "Expected 401 for expired token" || test_result=1
    assert_json_contains "error" "$response_file" "Response missing error details" || test_result=1
    end_test $test_result "Validation failed"
    return $test_result
}

test_middleware_auth_invalid_signing() {
    begin_test "JWT Middleware: Invalid signing method"
    local test_result=0
    local response_file="response_$$.txt"
    local status

    # Generate token with different secret
    local token=$(jwt "invalid_secret_32_bytes_long_xxxxxx" "middleware_invalid_sig" "+5 minutes")
    
    http_request GET "$PROTECTED_ENDPOINT" status "$response_file" "" \
        "Authorization: Bearer $token"
    local request_status=$?

    if [ $request_status -ne 0 ]; then
        end_test 1 "Curl command failed with exit code $request_status"
        return 1
    fi

    assert_status 401 "$status" "Expected 401 for invalid signature" || test_result=1
    assert_json_contains "error" "$response_file" "Response missing error details" || test_result=1
    end_test $test_result "Validation failed"
    return $test_result
}

main() {
    validate_environment
    
    # Setup test database
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
    test_middleware_auth_valid
    test_middleware_auth_missing_header
    test_middleware_auth_invalid_format
    test_middleware_auth_expired_token
    test_middleware_auth_invalid_signing
    
    print_test_summary
    stop_server "$server_pid"
    cleanup "$db_file"
}

main
