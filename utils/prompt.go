package utils

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/manifoldco/promptui"
)

// Generic prompt function
func PromptForSelection(label string, options []string) string {
	prompt := promptui.Select{
		Label: label,
		Items: options,
	}

	_, result, err := prompt.Run()
	if err != nil {
		log.Fatalf("Prompt failed: %v\n", err)
	}
	return result
}

// AWS Region selection

func PromptForRegion() (string, error) {
	regions := []string{"us-east-1", "us-west-2", "eu-central-1"}
	// defaultRegion := "us-east-1"

	prompt := promptui.Select{
		Label: "Choose region",
		Items: regions,
		Size:  len(regions),
	}

	_, selectedRegion, err := prompt.Run()
	// if err != nil {
	// 	return defaultRegion // If selection fails, return default
	// }
	if err != nil {
		return "", fmt.Errorf("selection aborted")
	}

	return selectedRegion, nil
}

// PromptForRole prompts the user to select an AWS IAM role interactively.

func PromptForRole() (string, error) {
	roles := []string{"AdministratorAccess", "Billing", "PowerUserAccess", "ViewOnlyAccess", "LogsReadOnlyPermissionSet", "S3FullAccess"}
	// defaultRole := "AdministratorAccess"

	prompt := promptui.Select{
		Label: "Choose role",
		Items: roles,
		Size:  len(roles),
	}

	_, selectedRole, err := prompt.Run()
	// if err != nil {
	// 	return defaultRole // If selection fails, return default
	// }
	if err != nil {
		return "", fmt.Errorf("selection aborted")
	}

	return selectedRole, nil
}

// PromptForAccount prompts the user to select an AWS account interactively.

func PromptForAccount() (string, error) {
	// accounts := []string{"Ls", "NP", "on", "ty", "ices", "berg"}
	accounts := []string{"Logs", "NonProd", "Security", "Production", "Shared Services", "MarcRosenberg"}
	// defaultAccount := "Production"

	prompt := promptui.Select{
		Label: "Choose account",
		Items: accounts,
		Size:  len(accounts),
	}

	_, selected, err := prompt.Run()
	// if err != nil {
	// 	return defaultAccount // If selection fails, return default
	// }
	if err != nil {
		return "", fmt.Errorf("selection aborted")
	}

	return selected, nil
}

// PromptForProfile lets the user choose an AWS profile interactively
func PromptForProfile() string {
	// Run `aws configure list-profiles` to get the list of profiles
	cmd := exec.Command("aws", "configure", "list-profiles")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("❌ Error: AWS CLI not found or misconfigured.")
		os.Exit(1)
	}

	// Convert output into a slice of profile names
	profiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(profiles) == 0 {
		fmt.Println("❌ No AWS profiles found. Run `aws configure` to set up.")
		os.Exit(1)
	}

	// Define a default profile (ensure it exists in the list)
	defaultProfile := "NonProd"
	defaultIndex := 0
	for i, profile := range profiles {
		if profile == defaultProfile {
			defaultIndex = i
			break
		}
	}

	// Ensure CursorPos is within the valid range
	if defaultIndex >= len(profiles) {
		defaultIndex = 0
	}

	// Prompt user to select a profile with the default selected
	prompt := promptui.Select{
		Label:     "Choose AWS Profile",
		Items:     profiles,
		Size:      15,
		CursorPos: defaultIndex,
	}

	_, selectedProfile, err := prompt.Run()
	if err != nil {
		fmt.Println("❌ Profile selection cancelled.")
		os.Exit(1)
	}

	// Set AWS_PROFILE environment variable
	os.Setenv("AWS_PROFILE", selectedProfile)
	fmt.Println("Run this command in your shell to persist the profile:")
	fmt.Printf("export AWS_PROFILE=%s\n", selectedProfile)
	// fmt.Printf("✅ AWS Profile set: %s (Index: %d)\n", selectedProfile, selectedIndex)

	return selectedProfile
}

func PromptForConfirmation(prompt string) bool {
	promptInstance := promptui.Prompt{
		Label:     prompt,
		IsConfirm: true,
	}

	result, err := promptInstance.Run()
	if err != nil {
		// If the user exits the prompt, return false
		return false
	}

	// Normalize the input to lowercase and check if it starts with 'y' (yes)
	return strings.HasPrefix(strings.ToLower(result), "y")
}
