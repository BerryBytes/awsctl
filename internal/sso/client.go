package sso

import (
	generalUtils "github.com/BerryBytes/awsctl/utils/general"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
)

type AWSClient struct {
	ConfigClient      AWSConfigClient
	SSOClient         AWSSSOClient
	CredentialsClient AWSCredentialsClient
	SelectionClient   AWSSelectionClient
	UtilityClient     AWSUtilityClient
}

func NewAWSClient(
	configClient AWSConfigClient,
	ssoClient AWSSSOClient,
	credentialsClient AWSCredentialsClient,
	selectionClient AWSSelectionClient,
	utilityClient AWSUtilityClient,
) AWSClient {
	return AWSClient{
		ConfigClient:      configClient,
		SSOClient:         ssoClient,
		CredentialsClient: credentialsClient,
		SelectionClient:   selectionClient,
		UtilityClient:     utilityClient,
	}
}

func DefaultAWSClient() AWSClient {
	executor := &RealCommandExecutor{}

	ssoClient, _ := NewRealAWSSSOClient(executor)

	return NewAWSClient(
		&RealAWSConfigClient{Executor: executor},
		ssoClient,
		ssoClient.CredentialsClient,
		&RealAWSSelectionClient{Prompter: promptUtils.NewPrompt()},
		&RealAWSUtilityClient{GeneralManager: generalUtils.NewGeneralUtilsManager()},
	)
}
