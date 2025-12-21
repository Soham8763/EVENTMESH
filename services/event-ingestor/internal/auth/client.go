package auth

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Client struct {
	baseURL string
}

func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL}
}

func (c *Client) ValidateAPIKey(apiKey string) (string, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		c.baseURL+"/validate",
		nil,
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("X-API-Key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("invalid api key")
	}

	var body struct {
		TenantID string `json:"tenant_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}

	return body.TenantID, nil
}
