package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/nbitslabs/agentchat/internal/apiclient"
	"github.com/nbitslabs/agentchat/internal/credstore"
)

type wsFrame struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type messagePayload struct {
	MessageID   string `json:"message_id"`
	SenderID    string `json:"sender_id"`
	RecipientID string `json:"recipient_id"`
	Type        string `json:"type"`
	Content     struct {
		Text string `json:"text"`
	} `json:"content"`
	CreatedAt   string `json:"created_at"`
	DeliveredAt string `json:"delivered_at,omitempty"`
}

func cmdListen() {
	store, err := credstore.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	token, err := ensureSession(store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	client := apiclient.New()
	wsURL := client.WebSocketURL(token)

	// Try WebSocket first
	conn, _, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "WebSocket connection failed, falling back to polling mode")
		pollMode(store)
		return
	}
	defer conn.Close()

	fmt.Fprintln(os.Stderr, "Connected. Listening for messages... (Ctrl+C to stop)")

	// Handle Ctrl+C
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var frame wsFrame
			if err := json.Unmarshal(msg, &frame); err != nil {
				continue
			}

			if frame.Type == "message" {
				var payload messagePayload
				if err := json.Unmarshal(frame.Payload, &payload); err != nil {
					continue
				}
				displayMessage(payload)

				// Mark as read asynchronously
				go func(id string) {
					t, _ := ensureSession(store)
					apiclient.New().MarkRead(t, id)
				}(payload.MessageID)
			}
		}
	}()

	select {
	case <-done:
		fmt.Fprintln(os.Stderr, "\nConnection closed. Reconnecting...")
		// Simple reconnect - in production you'd use exponential backoff
		cmdListen()
	case <-interrupt:
		fmt.Fprintln(os.Stderr, "\nShutting down...")
		conn.WriteMessage(ws.CloseMessage, ws.FormatCloseMessage(ws.CloseNormalClosure, ""))
	}
}

func pollMode(store *credstore.Store) {
	fmt.Fprintln(os.Stderr, "[polling mode] Checking for messages every 5 seconds... (Ctrl+C to stop)")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Poll immediately first
	pollOnce(store)

	for {
		select {
		case <-ticker.C:
			pollOnce(store)
		case <-interrupt:
			fmt.Fprintln(os.Stderr, "\nShutting down...")
			return
		}
	}
}

func pollOnce(store *credstore.Store) {
	token, err := ensureSession(store)
	if err != nil {
		log.Printf("session error: %v", err)
		return
	}

	client := apiclient.New()
	resp, err := client.PollMessages(token)
	if err != nil {
		log.Printf("poll error: %v", err)
		return
	}

	if !resp.Success {
		return
	}

	var data struct {
		Messages []messagePayload `json:"messages"`
	}
	json.Unmarshal(resp.Data, &data)

	for _, msg := range data.Messages {
		displayMessage(msg)
		go func(id string) {
			t, _ := ensureSession(store)
			apiclient.New().MarkRead(t, id)
		}(msg.MessageID)
	}
}

func displayMessage(msg messagePayload) {
	ts := msg.CreatedAt
	if t, err := time.Parse(time.RFC3339, msg.CreatedAt); err == nil {
		ts = t.Local().Format("2006-01-02 15:04:05")
	}
	fmt.Printf("[%s] From %s: %s\n", ts, msg.SenderID, msg.Content.Text)
}
