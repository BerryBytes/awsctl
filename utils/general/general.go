package generalutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

type GeneralUtilsInterface interface {
	CheckAWSCLI() error
	HandleSignals() context.Context
	PrintCurrentRole(profile, accountID, accountName, roleName, roleARN, expiration string)
}

type DefaultGeneralUtilsManager struct{}

func (d *DefaultGeneralUtilsManager) CheckAWSCLI() error {
	_, err := exec.LookPath("aws")
	if err != nil {
		return fmt.Errorf("AWS CLI not found: %w", err)
	}
	return nil
}

func (g *DefaultGeneralUtilsManager) HandleSignals() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("Received termination signal: %v\n", sig)
		cancel()
	}()

	return ctx
}

func (d *DefaultGeneralUtilsManager) PrintCurrentRole(profile, accountID, accountName, roleName, roleARN, expiration string) {
	fmt.Printf(`
AWS Session Details:
---------------------------------
Profile      : %s
Account Id   : %s
Account Name : %s
Role Name    : %s
Role ARN     : %s
Expiration   : %s
---------------------------------
`, profile, accountID, accountName, roleName, roleARN, expiration)
}

func NewGeneralUtilsManager() GeneralUtilsInterface {
	return &DefaultGeneralUtilsManager{}
}
