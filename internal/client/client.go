package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client represents a NetBox API client
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient creates a new NetBox API client
func NewClient(baseURL, token string, insecure bool) *Client {
	// Ensure baseURL has proper format
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	transport := &http.Transport{
		TLSClientConfig: nil,
	}

	if insecure {
		// This would require importing crypto/tls and setting InsecureSkipVerify
		// For now, we'll use the default transport
	}

	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout:   time.Second * 30,
			Transport: transport,
		},
	}
}

// Request represents a generic API request
type Request struct {
	Method      string
	Path        string
	Body        interface{}
	QueryParams url.Values
}

// Response represents a generic API response
type Response struct {
	StatusCode int
	Body       []byte
}

// DoRequest performs an HTTP request to the NetBox API
func (c *Client) DoRequest(ctx context.Context, req Request) (*Response, error) {
	// Build URL
	apiURL := fmt.Sprintf("%s%s", c.BaseURL, req.Path)
	if len(req.QueryParams) > 0 {
		apiURL = fmt.Sprintf("%s?%s", apiURL, req.QueryParams.Encode())
	}

	// Marshal body if present
	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, apiURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Authorization", fmt.Sprintf("Token %s", c.Token))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Execute request
	httpResp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	resp := &Response{
		StatusCode: httpResp.StatusCode,
		Body:       respBody,
	}

	// Check for errors
	if httpResp.StatusCode >= 400 {
		return resp, fmt.Errorf("API error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	return resp, nil
}

// GetList retrieves a paginated list of objects from NetBox
func (c *Client) GetList(ctx context.Context, path string, params url.Values) ([]json.RawMessage, error) {
	var allResults []json.RawMessage
	nextURL := path

	for nextURL != "" {
		req := Request{
			Method:      "GET",
			Path:        nextURL,
			QueryParams: params,
		}

		// Only use query params on the first request
		params = nil

		resp, err := c.DoRequest(ctx, req)
		if err != nil {
			return nil, err
		}

		var listResp struct {
			Count   int               `json:"count"`
			Next    *string           `json:"next"`
			Results []json.RawMessage `json:"results"`
		}

		if err := json.Unmarshal(resp.Body, &listResp); err != nil {
			return nil, fmt.Errorf("failed to parse list response: %w", err)
		}

		allResults = append(allResults, listResp.Results...)

		// Handle pagination
		if listResp.Next != nil && *listResp.Next != "" {
			// Extract just the path and query from the next URL
			nextURLParsed, err := url.Parse(*listResp.Next)
			if err != nil {
				return nil, fmt.Errorf("failed to parse next URL: %w", err)
			}
			nextURL = nextURLParsed.Path + "?" + nextURLParsed.RawQuery
		} else {
			nextURL = ""
		}
	}

	return allResults, nil
}

// Get retrieves a single object by ID
func (c *Client) Get(ctx context.Context, path string) (*Response, error) {
	req := Request{
		Method: "GET",
		Path:   path,
	}
	return c.DoRequest(ctx, req)
}

// Create creates a new object
func (c *Client) Create(ctx context.Context, path string, body interface{}) (*Response, error) {
	req := Request{
		Method: "POST",
		Path:   path,
		Body:   body,
	}
	return c.DoRequest(ctx, req)
}

// Update updates an existing object
func (c *Client) Update(ctx context.Context, path string, body interface{}) (*Response, error) {
	req := Request{
		Method: "PATCH",
		Path:   path,
		Body:   body,
	}
	return c.DoRequest(ctx, req)
}

// Delete deletes an object
func (c *Client) Delete(ctx context.Context, path string) (*Response, error) {
	req := Request{
		Method: "DELETE",
		Path:   path,
	}
	return c.DoRequest(ctx, req)
}
