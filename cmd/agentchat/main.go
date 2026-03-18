package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "auth":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: agentchat auth <generate|import|status>")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "generate":
			cmdAuthGenerate()
		case "import":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Usage: agentchat auth import <key-file>")
				os.Exit(1)
			}
			cmdAuthImport(os.Args[3])
		case "status":
			cmdAuthStatus()
		default:
			fmt.Fprintf(os.Stderr, "Unknown auth command: %s\n", os.Args[2])
			os.Exit(1)
		}
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`agentchat - Agent Chat CLI

Commands:
  auth generate              Generate a new cryptographic identity
  auth import <key-file>     Import an existing private key
  auth status                Display identity and session status
  help                       Show this help message`)
}
