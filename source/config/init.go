package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// RunInit runs the interactive init flow, prompting the user for each config value.
func RunInit() error {
	reader := bufio.NewReader(os.Stdin)

	// Load current effective config as defaults
	current := DefaultConfig()

	fmt.Println("🔧 ai-reviewer init")
	fmt.Println("Configure your defaults. Press Enter to keep the current value.")
	fmt.Println(strings.Repeat("─", 50))

	// Non-secret, file-persisted values
	current.ModelEndpoint = prompt(reader, "Model endpoint", current.ModelEndpoint)
	current.Model = prompt(reader, "Model name", current.Model)
	current.PromptExtra = prompt(reader, "Extra prompt directives", current.PromptExtra)
	current.BBWorkspace = prompt(reader, "Default Bitbucket workspace", current.BBWorkspace)
	current.BBEmail = prompt(reader, "Bitbucket email", current.BBEmail)
	current.Pending = promptBool(reader, "Include \"pending\" in comment payloads", current.Pending)

	// Save non-secret config to file
	if err := SaveConfigFile(current); err != nil {
		return fmt.Errorf("saving config file: %w", err)
	}
	fmt.Printf("   ✅ Config saved to %s\n", ConfigFilePath())

	// Secret values → keyring
	apiKey := promptSecret(reader, "LLM API key", current.APIKey)
	if apiKey != "" {
		if err := SaveSecret(KeyAPIKey, apiKey); err != nil {
			return fmt.Errorf("saving API key to keyring: %w", err)
		}
		fmt.Println("   ✅ API key saved to system keyring")
	}

	bbToken := promptSecret(reader, "Bitbucket API token", current.BBToken)
	if bbToken != "" {
		if err := SaveSecret(KeyBBToken, bbToken); err != nil {
			return fmt.Errorf("saving Bitbucket token to keyring: %w", err)
		}
		fmt.Println("   ✅ Bitbucket token saved to system keyring")
	}

	fmt.Println()
	fmt.Println("✅ Configuration complete! You can re-run 'ai-reviewer init' at any time to update.")
	return nil
}

// prompt asks the user for a string value, showing the current default.
func prompt(reader *bufio.Reader, label, current string) string {
	display := current
	if display == "" {
		display = "(none)"
	}
	fmt.Printf("  %s [%s]: ", label, display)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return current
	}
	return line
}

// promptSecret asks for a secret value, masking the current value.
func promptSecret(reader *bufio.Reader, label, current string) string {
	display := "(not set)"
	if current != "" {
		display = "********"
	}
	fmt.Printf("  %s [%s]: ", label, display)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	return line
}

// promptBool asks for a boolean value.
func promptBool(reader *bufio.Reader, label string, current bool) bool {
	currentStr := "no"
	if current {
		currentStr = "yes"
	}
	fmt.Printf("  %s [%s]: ", label, currentStr)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return current
	}
	return line == "yes" || line == "y" || line == "true" || line == "1"
}
