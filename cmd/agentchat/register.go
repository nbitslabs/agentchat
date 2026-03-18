package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nbitslabs/agentchat/internal/apiclient"
	"github.com/nbitslabs/agentchat/internal/credstore"
)

func cmdRegister() {
	store, err := credstore.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !store.HasIdentity() {
		fmt.Fprintln(os.Stderr, "No identity found. Run 'agentchat auth generate' first.")
		os.Exit(1)
	}

	identity, err := store.LoadIdentity()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading identity: %v\n", err)
		os.Exit(1)
	}

	client := apiclient.New()
	resp, err := client.Register(identity.RootPublicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Registration failed: %s\n", resp.Error.Message)
		os.Exit(1)
	}

	var data struct {
		AgentID string `json:"agent_id"`
	}
	json.Unmarshal(resp.Data, &data)

	fmt.Println("Registration successful!")
	fmt.Printf("  Agent ID: %s\n", data.AgentID)
}
