package sso

import (
	generalutils "awsctl/utils/general"
	"fmt"
)

type AWSUtilityClient interface {
	AbortSetup(err error) error
	PrintCurrentRole(profile, accountID, accountName, roleName, roleARN, expiration string)
}

type RealAWSUtilityClient struct {
	GeneralManager generalutils.GeneralUtilsInterface
}

func (c *RealAWSUtilityClient) AbortSetup(err error) error {
	fmt.Println("Setup aborted. No changes made.")
	return err
}

func (c *RealAWSUtilityClient) PrintCurrentRole(profile, accountID, accountName, roleName, roleARN, expiration string) {
	c.GeneralManager.PrintCurrentRole(profile, accountID, accountName, roleName, roleARN, expiration)
}
