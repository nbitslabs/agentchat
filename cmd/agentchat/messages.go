package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/nbitslabs/agentchat/internal/apiclient"
	"github.com/nbitslabs/agentchat/internal/credstore"
)

func cmdMessagesHistory(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: agentchat messages history <partner-id> [--limit N] [--offset N]")
		os.Exit(1)
	}

	partnerID := args[0]
	limit := 50
	offset := 0
	jsonOutput := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--limit":
			if i+1 < len(args) {
				limit, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "--offset":
			if i+1 < len(args) {
				offset, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "--json":
			jsonOutput = true
		}
	}

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

	identity, _ := store.LoadIdentity()
	myID := ""
	if identity != nil {
		myID = identity.AgentID
	}

	client := apiclient.New()
	resp, err := client.GetHistory(token, partnerID, limit, offset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error.Message)
		os.Exit(1)
	}

	if jsonOutput {
		fmt.Println(string(resp.Data))
		return
	}

	var data struct {
		Messages []struct {
			MessageID   string `json:"message_id"`
			SenderID    string `json:"sender_id"`
			RecipientID string `json:"recipient_id"`
			Content     struct {
				Text string `json:"text"`
			} `json:"content"`
			CreatedAt   string `json:"created_at"`
			DeliveredAt string `json:"delivered_at,omitempty"`
			ReadAt      string `json:"read_at,omitempty"`
		} `json:"messages"`
		Total int `json:"total"`
	}
	json.Unmarshal(resp.Data, &data)

	if len(data.Messages) == 0 {
		fmt.Println("No messages in this conversation.")
		return
	}

	fmt.Printf("Conversation with %s (%d total messages)\n\n", partnerID, data.Total)
	for _, msg := range data.Messages {
		ts := msg.CreatedAt
		if t, err := time.Parse(time.RFC3339, msg.CreatedAt); err == nil {
			ts = t.Local().Format("2006-01-02 15:04:05")
		}

		direction := "<<<"
		if msg.SenderID == myID {
			direction = ">>>"
		}
		fmt.Printf("[%s] %s %s\n", ts, direction, msg.Content.Text)
	}
}

func cmdMessagesList() {
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
	resp, err := client.ListConversations(token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error.Message)
		os.Exit(1)
	}

	var data struct {
		Conversations []struct {
			PartnerID     string `json:"partner_id"`
			Username      string `json:"username,omitempty"`
			LastMessageAt string `json:"last_message_at"`
			Preview       string `json:"preview"`
		} `json:"conversations"`
	}
	json.Unmarshal(resp.Data, &data)

	if len(data.Conversations) == 0 {
		fmt.Println("No conversations yet.")
		return
	}

	fmt.Println("Conversations:")
	for _, c := range data.Conversations {
		ts := c.LastMessageAt
		if t, err := time.Parse(time.RFC3339, c.LastMessageAt); err == nil {
			ts = t.Local().Format("2006-01-02 15:04:05")
		}

		name := c.PartnerID
		if c.Username != "" {
			name = fmt.Sprintf("%s (%s)", c.Username, c.PartnerID)
		}

		preview := c.Preview
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		fmt.Printf("  [%s] %s: %s\n", ts, name, preview)
	}
}
