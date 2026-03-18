package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nbitslabs/agentchat/internal/apiclient"
)

type agentProfile struct {
	AgentID       string `json:"agent_id"`
	Username      string `json:"username,omitempty"`
	RootPublicKey string `json:"root_public_key"`
	Fingerprint   string `json:"fingerprint"`
}

func cmdDiscoverSearch(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: agentchat discover search <query> [--json] [--full-key]")
		os.Exit(1)
	}

	query := args[0]
	jsonOutput := false
	fullKey := false
	for _, arg := range args[1:] {
		switch arg {
		case "--json":
			jsonOutput = true
		case "--full-key":
			fullKey = true
		}
	}

	client := apiclient.New()
	resp, err := client.SearchAgents(query, 20, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error.Message)
		os.Exit(1)
	}

	var data struct {
		Agents []agentProfile `json:"agents"`
	}
	json.Unmarshal(resp.Data, &data)

	if jsonOutput {
		out, _ := json.MarshalIndent(data.Agents, "", "  ")
		fmt.Println(string(out))
		return
	}

	if len(data.Agents) == 0 {
		fmt.Println("No agents found matching your query.")
		return
	}

	printAgentTable(data.Agents, fullKey)
}

func cmdDiscoverLookup(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: agentchat discover lookup <agent-id> [--json] [--full-key]")
		os.Exit(1)
	}

	agentID := args[0]
	jsonOutput := false
	fullKey := false
	for _, arg := range args[1:] {
		switch arg {
		case "--json":
			jsonOutput = true
		case "--full-key":
			fullKey = true
		}
	}

	client := apiclient.New()
	resp, err := client.LookupAgent(agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error.Message)
		os.Exit(1)
	}

	var profile agentProfile
	json.Unmarshal(resp.Data, &profile)

	if jsonOutput {
		out, _ := json.MarshalIndent(profile, "", "  ")
		fmt.Println(string(out))
		return
	}

	fmt.Println("Agent Profile:")
	fmt.Printf("  Agent ID:    %s\n", profile.AgentID)
	if profile.Username != "" {
		fmt.Printf("  Username:    %s\n", profile.Username)
	} else {
		fmt.Printf("  Username:    (none)\n")
	}
	fmt.Printf("  Fingerprint: %s\n", profile.Fingerprint)
	if fullKey {
		fmt.Printf("  Public Key:  %s\n", profile.RootPublicKey)
	}
}

func cmdDiscoverList(args []string) {
	limit := 20
	offset := 0
	jsonOutput := false
	fullKey := false
	fetchAll := false

	for i := 0; i < len(args); i++ {
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
		case "--full-key":
			fullKey = true
		case "--all":
			fetchAll = true
		}
	}

	client := apiclient.New()

	if fetchAll {
		var allAgents []agentProfile
		page := 0
		for {
			resp, err := client.ListDirectory(100, page*100)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			var data struct {
				Agents []agentProfile `json:"agents"`
				Total  int            `json:"total"`
			}
			json.Unmarshal(resp.Data, &data)
			allAgents = append(allAgents, data.Agents...)
			fmt.Fprintf(os.Stderr, "\rFetched %d / %d agents...", len(allAgents), data.Total)
			if len(allAgents) >= data.Total || len(data.Agents) == 0 {
				break
			}
			page++
		}
		fmt.Fprintln(os.Stderr)

		if jsonOutput {
			out, _ := json.MarshalIndent(allAgents, "", "  ")
			fmt.Println(string(out))
		} else {
			printAgentTable(allAgents, fullKey)
		}
		return
	}

	resp, err := client.ListDirectory(limit, offset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error.Message)
		os.Exit(1)
	}

	var data struct {
		Agents []agentProfile `json:"agents"`
		Total  int            `json:"total"`
	}
	json.Unmarshal(resp.Data, &data)

	if jsonOutput {
		out, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(out))
		return
	}

	if len(data.Agents) == 0 {
		fmt.Println("No agents in the directory.")
		return
	}

	fmt.Printf("Agent Directory (%d total)\n\n", data.Total)
	printAgentTable(data.Agents, fullKey)
}

func cmdDiscoverVerify(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: agentchat discover verify <agent-id> <fingerprint>")
		os.Exit(1)
	}

	agentID := args[0]
	expectedFP := args[1]

	client := apiclient.New()
	resp, err := client.LookupAgent(agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error.Message)
		os.Exit(1)
	}

	var profile agentProfile
	json.Unmarshal(resp.Data, &profile)

	if strings.EqualFold(profile.Fingerprint, expectedFP) {
		fmt.Println("VERIFIED: Fingerprint matches!")
		fmt.Printf("  Agent ID:    %s\n", profile.AgentID)
		fmt.Printf("  Fingerprint: %s\n", profile.Fingerprint)
	} else {
		fmt.Println("WARNING: Fingerprint does NOT match!")
		fmt.Printf("  Agent ID:    %s\n", profile.AgentID)
		fmt.Printf("  Expected:    %s\n", expectedFP)
		fmt.Printf("  Actual:      %s\n", profile.Fingerprint)
		os.Exit(1)
	}
}

func printAgentTable(agents []agentProfile, fullKey bool) {
	// Header
	fmt.Printf("%-40s  %-20s  %s\n", "AGENT ID", "USERNAME", "FINGERPRINT")
	fmt.Printf("%-40s  %-20s  %s\n", strings.Repeat("-", 40), strings.Repeat("-", 20), strings.Repeat("-", 24))

	for _, a := range agents {
		username := a.Username
		if username == "" {
			username = "-"
		}
		fp := a.Fingerprint
		if !fullKey && len(fp) > 24 {
			fp = fp[:24] + "..."
		}
		fmt.Printf("%-40s  %-20s  %s\n", a.AgentID, username, fp)
	}
}
