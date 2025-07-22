package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJsonWithData(t *testing.T) {
	type testData struct {
		Name string `json:"name"`
	}

	testCases := []struct {
		name           string
		resp           JsonWithData
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Basic Response",
			resp: JsonWithData{
				JsonBasic: JsonBasic{
					Status:  http.StatusOK,
					Code:    "ok",
					Message: "Success",
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":200,"code":"ok","message":"Success"}`,
		},
		{
			name: "Response with Struct Data",
			resp: JsonWithData{
				JsonBasic: JsonBasic{
					Status:  http.StatusOK,
					Code:    "ok_data",
					Message: "Data retrieved",
				},
				Data: testData{Name: "Test"},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":200,"code":"ok_data","message":"Data retrieved","data":{"name":"Test"}}`,
		},
		{
			name: "Response with Map Data",
			resp: JsonWithData{
				JsonBasic: JsonBasic{
					Status:  http.StatusCreated,
					Code:    "created",
					Message: "Resource created",
				},
				Data: map[string]interface{}{"id": 123, "active": true},
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"status":201,"code":"created","message":"Resource created","data":{"active":true,"id":123}}`,
		},
		{
			name: "Response with Slice Data",
			resp: JsonWithData{
				JsonBasic: JsonBasic{
					Status:  http.StatusOK,
					Code:    "ok_list",
					Message: "List of items",
				},
				Data: []string{"item1", "item2"},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":200,"code":"ok_list","message":"List of items","data":["item1","item2"]}`,
		},
		{
			name: "Response with Nil Data",
			resp: JsonWithData{
				JsonBasic: JsonBasic{
					Status:  http.StatusOK,
					Code:    "ok_nil",
					Message: "Success with nil data",
				},
				Data: nil,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":200,"code":"ok_nil","message":"Success with nil data"}`,
		},
		{
			name: "Error Response",
			resp: JsonWithData{
				JsonBasic: JsonBasic{
					Status:  http.StatusNotFound,
					Code:    "not_found",
					Message: "The requested resource was not found.",
				},
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `{"status":404,"code":"not_found","message":"The requested resource was not found."}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteJsonWithData(w, tc.resp)

			// Check status code
			if w.Code != tc.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					w.Code, tc.expectedStatus)
			}

			// Check header
			expectedHeader := "application/json; charset=utf-8"
			if contentType := w.Header().Get("Content-Type"); contentType != expectedHeader {
				t.Errorf("handler returned wrong content type: got %q want %q",
					contentType, expectedHeader)
			}

			// Compare body
			actualBody := strings.TrimSpace(w.Body.String())
			if actualBody != tc.expectedBody {
				t.Errorf("response body mismatch: got:  %s want: %s", actualBody, tc.expectedBody)
			}
		})
	}
}
