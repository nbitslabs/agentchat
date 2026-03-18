package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client wraps HTTP calls to the agentchat API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new API client. Uses AGENTCHAT_API_URL env var or defaults to http://localhost:8080.
func New() *Client {
	base := os.Getenv("AGENTCHAT_API_URL")
	if base == "" {
		base = "http://localhost:8080"
	}
	return &Client{
		baseURL: base,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// APIResponse is the standard response shape.
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *APIError       `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *Client) post(path string, body interface{}, token string) (*APIResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return c.do(req)
}

func (c *Client) do(req *http.Request) (*APIResponse, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("invalid response: %s", string(body))
	}

	return &apiResp, nil
}

// Register registers an agent with the given public key.
func (c *Client) Register(rootPublicKey string) (*APIResponse, error) {
	return c.post("/api/v1/agents/register", map[string]string{
		"root_public_key": rootPublicKey,
	}, "")
}

// CreateSession creates a new session.
func (c *Client) CreateSession(agentID, sessionPublicKey, signature string) (*APIResponse, error) {
	return c.post("/api/v1/sessions/create", map[string]string{
		"agent_id":           agentID,
		"session_public_key": sessionPublicKey,
		"signature":          signature,
	}, "")
}

// ClaimUsername claims a username for the agent.
func (c *Client) ClaimUsername(token, username string) (*APIResponse, error) {
	return c.post("/api/v1/agents/username/claim", map[string]string{
		"username": username,
	}, token)
}

// SendMessage sends a message to a recipient.
func (c *Client) SendMessage(token, recipient, content string) (*APIResponse, error) {
	return c.post("/api/v1/messages/send", map[string]string{
		"recipient": recipient,
		"content":   content,
	}, token)
}

// GetHistory fetches conversation history with a partner.
func (c *Client) GetHistory(token, partnerID string, limit, offset int) (*APIResponse, error) {
	return c.get(fmt.Sprintf("/api/v1/messages/history/%s?limit=%d&offset=%d", partnerID, limit, offset), token)
}

// ListConversations lists all conversations.
func (c *Client) ListConversations(token string) (*APIResponse, error) {
	return c.get("/api/v1/messages/conversations", token)
}

// PollMessages polls for undelivered messages.
func (c *Client) PollMessages(token string) (*APIResponse, error) {
	return c.get("/api/v1/messages/poll", token)
}

// MarkRead marks a message as read.
func (c *Client) MarkRead(token, messageID string) (*APIResponse, error) {
	return c.post("/api/v1/messages/mark-read", map[string]string{
		"message_id": messageID,
	}, token)
}

// WebSocketURL returns the WebSocket URL for the API.
func (c *Client) WebSocketURL(token string) string {
	wsBase := strings.Replace(c.baseURL, "http://", "ws://", 1)
	wsBase = strings.Replace(wsBase, "https://", "wss://", 1)
	return wsBase + "/api/v1/ws?token=" + token
}

func (c *Client) get(path string, token string) (*APIResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.do(req)
}
