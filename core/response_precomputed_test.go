package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestPrecomputeBasicResponse(t *testing.T) {
	testCases := []struct {
		name        string
		status      int
		code        string
		message     string
		wantStatus  int
		wantBody    JsonBasic
		expectError bool
	}{
		{
			name:       "Valid OK response",
			status:     http.StatusOK,
			code:       "ok_success",
			message:    "Operation was successful",
			wantStatus: http.StatusOK,
			wantBody: JsonBasic{
				Status:  http.StatusOK,
				Code:    "ok_success",
				Message: "Operation was successful",
			},
			expectError: false,
		},
		{
			name:       "Valid Error response",
			status:     http.StatusBadRequest,
			code:       "err_invalid_input",
			message:    "Invalid input provided",
			wantStatus: http.StatusBadRequest,
			wantBody: JsonBasic{
				Status:  http.StatusBadRequest,
				Code:    "err_invalid_input",
				Message: "Invalid input provided",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Test PrecomputeBasicResponse function
			precomputed := PrecomputeBasicResponse(tc.status, tc.code, tc.message)

			if precomputed.status != tc.wantStatus {
				t.Errorf("PrecomputeBasicResponse() status = %v, want %v", precomputed.status, tc.wantStatus)
			}

			var gotBody JsonBasic
			err := json.Unmarshal(precomputed.body, &gotBody)
			if err != nil {
				t.Fatalf("Failed to unmarshal precomputed body: %v", err)
			}

			if !reflect.DeepEqual(gotBody, tc.wantBody) {
				t.Errorf("PrecomputeBasicResponse() body = %+v, want %+v", gotBody, tc.wantBody)
			}

			// 2. Test response writer functions (WriteJsonOk and WriteJsonError)
			rr := httptest.NewRecorder()
			if tc.status >= 200 && tc.status < 300 {
				WriteJsonOk(rr, precomputed)
			} else {
				WriteJsonError(rr, precomputed)
			}

			if rr.Code != tc.wantStatus {
				t.Errorf("ResponseWriter status = %v, want %v", rr.Code, tc.wantStatus)
			}

			var responseBody JsonBasic
			err = json.Unmarshal(rr.Body.Bytes(), &responseBody)
			if err != nil {
				t.Fatalf("Failed to unmarshal response writer body: %v", err)
			}

			if !reflect.DeepEqual(responseBody, tc.wantBody) {
				t.Errorf("ResponseWriter body = %+v, want %+v", responseBody, tc.wantBody)
			}

			// Check that the content type header is set correctly
			expectedContentType := "application/json; charset=utf-8"
			if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
				t.Errorf("Content-Type header = %q, want %q", contentType, expectedContentType)
			}
		})
	}
}

// TestPrecomputedVariables ensures that the global precomputed variables are correct.
func TestPrecomputedVariables(t *testing.T) {
	testCases := []struct {
		name         string
		variable     jsonResponse
		expectedCode int
		expectedBody JsonBasic
	}{
		{
			name:         "errorTokenGeneration",
			variable:     errorTokenGeneration,
			expectedCode: http.StatusInternalServerError,
			expectedBody: JsonBasic{
				Status:  http.StatusInternalServerError,
				Code:    CodeErrorTokenGeneration,
				Message: "Failed to generate authentication token",
			},
		},
		{
			name:         "okPasswordReset",
			variable:     okPasswordReset,
			expectedCode: http.StatusOK,
			expectedBody: JsonBasic{
				Status:  http.StatusOK,
				Code:    CodeOkPasswordReset,
				Message: "Password reset successfully",
			},
		},
		{
			name:         "errorInvalidRequest",
			variable:     errorInvalidRequest,
			expectedCode: http.StatusBadRequest,
			expectedBody: JsonBasic{
				Status:  http.StatusBadRequest,
				Code:    CodeErrorInvalidRequest,
				Message: "The request contains invalid data",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.variable.status != tc.expectedCode {
				t.Errorf("variable.status = %d, want %d", tc.variable.status, tc.expectedCode)
			}

			var gotBody JsonBasic
			err := json.Unmarshal(tc.variable.body, &gotBody)
			if err != nil {
				t.Fatalf("Failed to unmarshal variable body: %v", err)
			}

			if !reflect.DeepEqual(gotBody, tc.expectedBody) {
				t.Errorf("variable body = %+v, want %+v", gotBody, tc.expectedBody)
			}
		})
	}
}
