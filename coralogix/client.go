package coralogix

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"
)

// Client for Coralogix API
type Client struct {
	url     string
	apiKey  string
	timeout int
	client  *http.Client
}

// NewClient configures and returns Coralogix API client
func NewClient(url string, apiKey string, timeout int) (interface{}, error) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	return &Client{url, apiKey, timeout, client}, nil
}

// Request executes request to Coralogix API
func (c *Client) Request(method string, path string, body interface{}) (map[string]interface{}, error) {
	var request *http.Request

	if body != nil {
		bodyJSON, err := json.Marshal(body)
		test := string(bodyJSON)
		print(test)
		if err != nil {
			return nil, err
		}
		request, _ = http.NewRequest(method, c.url+path, bytes.NewBuffer(bodyJSON))
	} else {
		request, _ = http.NewRequest(method, c.url+path, nil)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("Authorization", "Bearer "+c.apiKey)

	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	responseBytes, _ := ioutil.ReadAll(response.Body)

	if response.StatusCode == 200 || response.StatusCode == 201 {
		if method != "DELETE" && len(responseBytes) > 0 {
			var responseJSON interface{}

			err = json.Unmarshal(responseBytes, &responseJSON)
			if err != nil {
				return nil, nil
			}

			return responseJSON.(map[string]interface{}), nil
		}
		return nil, nil
	}

	return nil, errors.New("API Error: " + string(responseBytes))
}

// Get executes GET request to Coralogix API
func (c *Client) Get(path string) (map[string]interface{}, error) {
	return c.Request("GET", path, nil)
}

// Post executes POST request to Coralogix API
func (c *Client) Post(path string, body interface{}) (map[string]interface{}, error) {
	return c.Request("POST", path, body)
}

// Put executes PUT request to Coralogix API
func (c *Client) Put(path string, body interface{}) (map[string]interface{}, error) {
	return c.Request("PUT", path, body)
}

// Delete executes DELETE request to Coralogix API
func (c *Client) Delete(path string) (map[string]interface{}, error) {
	return c.Request("DELETE", path, nil)
}
