package REST

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// Client for Coralogix API
type Client struct {
	url    string
	apiKey string
	client *http.Client
}

func NewRESTClient(url string, apiKey string) *Client {
	return &Client{url, apiKey, &http.Client{}}
}

// Request executes request to Coralogix API
func (c *Client) Request(ctx context.Context, method string, path string, body interface{}) (map[string]interface{}, error) {
	var request *http.Request

	if body != nil {
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		request, _ = http.NewRequest(method, c.url+path, bytes.NewBuffer(bodyJSON))
	} else {
		request, _ = http.NewRequest(method, c.url+path, nil)
	}

	request = request.WithContext(ctx)

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("Authorization", "Bearer "+c.apiKey)

	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	responseBytes, _ := io.ReadAll(response.Body)

	if response.StatusCode == 200 || response.StatusCode == 201 {
		if method != "DELETE" && len(responseBytes) > 0 {
			var responseJSON interface{}

			err = json.Unmarshal(responseBytes, &responseJSON)
			if err != nil {
				return nil, nil
			}
			// some responses are not map but dont fail, just return nil
			if _, ok := responseJSON.(map[string]interface{}); ok {
				return responseJSON.(map[string]interface{}), nil
			} else {
				return nil, nil
			}
		}
		return nil, nil
	}

	return nil, errors.New("API Error: " + string(responseBytes))
}

// Get executes GET request to Coralogix API
func (c *Client) Get(ctx context.Context, path string) (map[string]interface{}, error) {
	return c.Request(ctx, "GET", path, nil)
}

// Post executes POST request to Coralogix API
func (c *Client) Post(ctx context.Context, path string, body interface{}) (map[string]interface{}, error) {
	return c.Request(ctx, "POST", path, body)
}

// Put executes PUT request to Coralogix API
func (c *Client) Put(ctx context.Context, path string, body interface{}) (map[string]interface{}, error) {
	return c.Request(ctx, "PUT", path, body)
}

// Delete executes DELETE request to Coralogix API
func (c *Client) Delete(ctx context.Context, path string) (map[string]interface{}, error) {
	return c.Request(ctx, "DELETE", path, nil)
}
