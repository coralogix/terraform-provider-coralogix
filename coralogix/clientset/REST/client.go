package REST

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
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
func (c *Client) Request(method string, path string, body interface{}) (map[string]interface{}, error) {
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
