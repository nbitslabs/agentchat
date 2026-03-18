package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nbitslabs/agentchat/internal/apiclient"
	"github.com/nbitslabs/agentchat/internal/credstore"
)

const maxContentSize = 10 * 1024 * 1024 // 10MB

func cmdSend(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: agentchat send <recipient> [message]")
		fmt.Fprintln(os.Stderr, "       agentchat send <recipient> -i")
		fmt.Fprintln(os.Stderr, "       echo 'hello' | agentchat send <recipient>")
		os.Exit(1)
	}

	recipient := args[0]
	jsonOutput := false
	interactive := false
	var messageArgs []string

	for _, arg := range args[1:] {
		switch arg {
		case "--json":
			jsonOutput = true
		case "-i", "--interactive":
			interactive = true
		default:
			messageArgs = append(messageArgs, arg)
		}
	}

	var content string

	if interactive {
		// Interactive mode: read until EOF
		fmt.Fprintln(os.Stderr, "Enter message (Ctrl+D to send, or '.' on a line by itself):")
		content = readInteractive()
	} else if len(messageArgs) > 0 {
		// Argument mode
		content = strings.Join(messageArgs, " ")
	} else {
		// Stdin mode
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			os.Exit(1)
		}
		content = string(data)
	}

	content = strings.TrimSpace(content)
	if content == "" {
		fmt.Fprintln(os.Stderr, "Error: message content cannot be empty")
		os.Exit(1)
	}

	if len(content) > maxContentSize {
		fmt.Fprintln(os.Stderr, "Error: message exceeds 10MB size limit")
		os.Exit(1)
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

	client := apiclient.New()
	resp, err := client.SendMessage(token, recipient, content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Send failed: %s\n", resp.Error.Message)
		os.Exit(1)
	}

	var data struct {
		MessageID string `json:"message_id"`
		Timestamp string `json:"timestamp"`
	}
	json.Unmarshal(resp.Data, &data)

	if jsonOutput {
		out, _ := json.Marshal(data)
		fmt.Println(string(out))
	} else {
		fmt.Printf("Message sent!\n  ID: %s\n  Time: %s\n", data.MessageID, data.Timestamp)
	}
}

func readInteractive() string {
	scanner := bufio.NewScanner(os.Stdin)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "." {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
