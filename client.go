package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Client struct {
	token                string
	queryEndpoint        string
	subscriptionEndpoint string
}

func (client *Client) Query(query string) (map[string]interface{}, error) {
	// Generate JSON payload string
	postBody, err := json.Marshal(map[string]string{
		"query": query,
	})
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	httpClient := &http.Client{}
	req, err := http.NewRequest("POST", client.queryEndpoint, bytes.NewReader(postBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+client.token)

	// Send request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse response body to []byte
	rawBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var body struct {
		Data   map[string]interface{} `json:"data"`
		Errors *[]struct {
			Message string
		} `json:"errors"`
	}
	err = json.Unmarshal([]byte(rawBody), &body)
	if err != nil {
		return nil, err
	}
	if body.Errors != nil {
		errorString, err := json.Marshal(body.Errors)
		if err != nil {
			return nil, err
		} else {
			return nil, fmt.Errorf(string(errorString))
		}
	}

	return body.Data, nil
}

func (client *Client) Mutate(query string) (map[string]interface{}, error) {
	return client.Query(query)
}
