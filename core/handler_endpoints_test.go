package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

func TestListEndpointsHandler(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name               string
		endpoints          config.Endpoints
		expectedStatusCode int
		expectedBody       JsonWithData
	}{
		{
			name: "happy path with endpoints",
			endpoints: config.Endpoints{
				RefreshAuth:          "POST /api/refresh-auth",
				ListEndpoints:        "GET /api/endpoints",
				AuthWithPassword:     "POST /api/auth",
				RegisterWithPassword: "POST /api/register",
			},
			expectedStatusCode: http.StatusOK,
			expectedBody: JsonWithData{
				JsonBasic: JsonBasic{
					Status:  http.StatusOK,
					Code:    CodeOkEndpoints,
					Message: "List of all available endpoints",
				},
			},
		},
		{
			name:               "edge case with no endpoints",
			endpoints:          config.Endpoints{},
			expectedStatusCode: http.StatusOK,
			expectedBody: JsonWithData{
				JsonBasic: JsonBasic{
					Status:  http.StatusOK,
					Code:    CodeOkEndpoints,
					Message: "List of all available endpoints",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock App with the test case's config
			mockApp := &App{}
			mockApp.SetConfigProvider(config.NewProvider(&config.Config{
				Endpoints: tc.endpoints,
			}))

			// Create a request and response recorder
			req, err := http.NewRequest("GET", "/endpoints", nil)
			if err != nil {
				t.Fatalf("could not create request: %v", err)
			}
			rr := httptest.NewRecorder()

			// Call the handler
			handler := http.HandlerFunc(mockApp.ListEndpointsHandler)
			handler.ServeHTTP(rr, req)

			// Check the status code
			if status := rr.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}

			

			// Unmarshal the response body
			var actualBody JsonWithData
			if err := json.Unmarshal(rr.Body.Bytes(), &actualBody); err != nil {
				t.Fatalf("could not unmarshal response body: %v", err)
			}

			// Compare the basic fields
			if actualBody.Status != tc.expectedBody.Status ||
				actualBody.Code != tc.expectedBody.Code ||
				actualBody.Message != tc.expectedBody.Message {
				t.Errorf("handler returned unexpected body fields: got %+v want %+v", actualBody.JsonBasic, tc.expectedBody.JsonBasic)
			}

			// Marshal the 'Data' part of the actual response to compare it
			var actualEndpoints config.Endpoints
			dataBytes, err := json.Marshal(actualBody.Data)
			if err != nil {
				t.Fatalf("could not marshal actual data: %v", err)
			}
			if err := json.Unmarshal(dataBytes, &actualEndpoints); err != nil {
				t.Fatalf("could not unmarshal data into Endpoints struct: %v", err)
			}

			// Compare the Endpoints struct
			if !reflect.DeepEqual(actualEndpoints, tc.endpoints) {
				t.Errorf("handler returned unexpected data:\ngot:  %+v\nwant: %+v", actualEndpoints, tc.endpoints)
			}
		})
	}
}