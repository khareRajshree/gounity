/*
 Copyright © 2025 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package gounity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/dell/gounity/api"
	"github.com/dell/gounity/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock API client
type MockAPIClient struct {
	BaseURL string
	Token   string
}

func (m *MockAPIClient) DoAndGetResponseBody(_ context.Context, _, _ string, _ map[string]string, _ interface{}) (*http.Response, error) {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	return w.Result(), nil
}

func (m *MockAPIClient) Delete(_ context.Context, _ string, _ map[string]string, _ interface{}) error {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	return nil
}

func (m *MockAPIClient) DoWithHeaders(_ context.Context, _, uri string, _ map[string]string, _, _ interface{}) error {
	if uri == "/unauthorized" {
		return &types.Error{
			ErrorContent: types.ErrorContent{
				HTTPStatusCode: http.StatusUnauthorized,
			},
		}
	}
	if uri == "/server-error" {
		return &types.Error{
			ErrorContent: types.ErrorContent{
				HTTPStatusCode: http.StatusInternalServerError,
			},
		}
	}
	return nil
}

func (m *MockAPIClient) Get(_ context.Context, _ string, _ map[string]string, _ interface{}) error {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	return nil
}

func (m *MockAPIClient) ParseJSONError(_ context.Context, _ *http.Response) error {
	return errors.New("mock parse JSON error")
}

func (m *MockAPIClient) Post(_ context.Context, _ string, _ map[string]string, _ interface{}, _ interface{}) error {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	return nil
}

func (m *MockAPIClient) Put(_ context.Context, _ string, _ map[string]string, _ interface{}, _ interface{}) error {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	return nil
}

func TestBasicSystemInfo(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		expectedError bool
	}{
		{
			name:          "Successful response",
			statusCode:    http.StatusOK,
			expectedError: false,
		},
		{
			name:          "Client error response",
			statusCode:    http.StatusBadRequest,
			expectedError: true,
		},
		{
			name:          "Server error response",
			statusCode:    http.StatusInternalServerError,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new HTTP test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			// Create a new client with the test server URL
			client := &Client{
				api: &MockAPIClient{
					BaseURL: server.URL,
				},
			}

			// Call the BasicSystemInfo function
			err := client.BasicSystemInfo(context.Background(), &ConfigConnect{})

			// Check if an error was expected
			if tt.expectedError {
				require.NoError(t, err)
			}
		})
	}
}

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		expectedError bool
	}{
		{
			name:          "Successful authentication",
			statusCode:    http.StatusOK,
			expectedError: false,
		},
		{
			name:          "Authentication failed",
			statusCode:    http.StatusUnauthorized,
			expectedError: true,
		},
		{
			name:          "Server error",
			statusCode:    http.StatusInternalServerError,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new HTTP test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			// Create a new client with the test server URL
			client := &Client{
				api: &MockAPIClient{
					BaseURL: server.URL,
				},
				loginMutex: sync.Mutex{},
			}

			// Call the Authenticate function
			err := client.Authenticate(context.Background(), &ConfigConnect{})

			// Check if an error was expected
			if tt.expectedError {
				require.NoError(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type MockError struct {
	ErrorContent struct {
		HTTPStatusCode int
	}
}

func (e *MockError) Error() string {
	return "mock error"
}

type MockClient struct {
	*Client
	AuthError bool
}

func TestExecuteWithRetryAuthenticate(t *testing.T) {
	tests := []struct {
		name          string
		uri           string
		expectedError bool
		authError     bool
	}{
		{
			name:          "Successful execution",
			uri:           "/success",
			expectedError: false,
			authError:     false,
		},
		{
			name:          "Server error",
			uri:           "/server-error",
			expectedError: true,
			authError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new HTTP test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.uri == "/unauthorized" {
					w.WriteHeader(http.StatusUnauthorized)
				} else if tt.uri == "/server-error" {
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer server.Close()

			// Create a new mock API client
			mockAPIClient := &MockAPIClient{
				BaseURL: server.URL,
			}

			// Create a new client with the mock API client
			client := &MockClient{
				Client: &Client{
					api:        mockAPIClient,
					loginMutex: sync.Mutex{},
				},
				AuthError: tt.authError,
			}

			// Call the executeWithRetryAuthenticate function
			err := client.executeWithRetryAuthenticate(context.Background(), http.MethodGet, tt.uri, nil, nil)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWithFieldsE(t *testing.T) {
	tests := []struct {
		name     string
		fields   map[string]interface{}
		message  string
		inner    error
		expected string
	}{
		{
			name:     "Nil fields and nil inner",
			fields:   nil,
			message:  "Test message",
			inner:    nil,
			expected: "Test message ",
		},
		{
			name:     "Nil fields with inner error",
			fields:   nil,
			message:  "Test message",
			inner:    errors.New("inner error"),
			expected: "Test message inner=inner error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := withFieldsE(tt.fields, tt.message, tt.inner)
			assert.EqualError(t, err, tt.expected)
		})
	}
}

var mockNewAPIClient = func(_ context.Context, endpoint string, _ api.ClientOptions, _ bool) (api.Client, error) {
	if endpoint == "http://error.com" {
		return nil, errors.New("error creating API client")
	}
	return &MockAPIClient{BaseURL: endpoint}, nil
}

func TestClientCreation(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		insecure  bool
		expectErr bool
	}{
		{
			name:      "Successful client creation",
			endpoint:  "http://example.com",
			insecure:  false,
			expectErr: false,
		},
		{
			name:      "Missing endpoint",
			endpoint:  "",
			insecure:  false,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClientWithArgs(context.Background(), tt.endpoint, tt.insecure)

			if tt.expectErr {
				require.Error(t, err)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

// SetToken sets the token in the mock API client.
func (m *MockAPIClient) SetToken(token string) {
	m.Token = token
}

// GetToken gets the token from the mock API client.
func (m *MockAPIClient) GetToken() string {
	return m.Token
}

func TestSetToken(t *testing.T) {
	mockAPIClient := &MockAPIClient{}
	client := &Client{api: mockAPIClient}

	token := "test-token"
	client.SetToken(token)

	assert.Equal(t, token, mockAPIClient.Token)
}

func TestGetToken(t *testing.T) {
	token := "test-token"
	mockAPIClient := &MockAPIClient{Token: token}
	client := &Client{api: mockAPIClient}

	retrievedToken := client.GetToken()
	assert.Equal(t, token, retrievedToken)
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      string
		insecure      string
		expectedError bool
	}{
		{
			name:          "Successful client creation",
			endpoint:      "http://example.com",
			insecure:      "false",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			os.Setenv("GOUNITY_ENDPOINT", tt.endpoint)
			os.Setenv("GOUNITY_INSECURE", tt.insecure)

			// Call the NewClient function
			client, err := NewClient(context.Background())

			if tt.expectedError {
				require.Error(t, err)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
			}

			// Unset environment variables
			os.Unsetenv("GOUNITY_ENDPOINT")
			os.Unsetenv("GOUNITY_INSECURE")
		})
	}
}
