package aws

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Initialize AWS setup
func InitAWSSetup() {
	handleSignals()
	checkAWSCLI()
}

func handleSignals() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigs
		fmt.Println("\nReceived termination signal. Cleaning up...")
		os.Exit(1) // Ensure immediate exit
	}()
}

// checkAWSCLI ensures AWS CLI is installed
func checkAWSCLI() {
	_, err := exec.LookPath("aws")
	if err != nil {
		fmt.Println("AWS CLI not found:", err)
		os.Exit(1)
	} else {
		fmt.Println("AWS CLI is installed and available in PATH.")
	}
}
