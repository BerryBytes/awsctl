package generalutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
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

func isValidRegionFormat(region string) bool {
	// Matches patterns like us-east-1, ap-southeast-2
	return regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d+$`).MatchString(region)
}

func IsRegionValid(region string) bool {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err == nil {
		ec2Client := ec2.NewFromConfig(cfg)
		output, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
			AllRegions: aws.Bool(true),
		})
		if err == nil {
			for _, r := range output.Regions {
				if r.RegionName != nil && *r.RegionName == region {
					return true
				}
			}
			return false
		}
	}

	return isValidRegionFormat(region)
}
