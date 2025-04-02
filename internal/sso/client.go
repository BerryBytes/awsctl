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

	return NewAWSClient(
		&RealAWSConfigClient{Executor: executor},
		&RealAWSSSOClient{CredentialsClient: &RealAWSCredentialsClient{configClient: &RealAWSConfigClient{}, executor: executor}},
		&RealAWSCredentialsClient{configClient: &RealAWSConfigClient{Executor: executor}, executor: executor},
		&RealAWSSelectionClient{Prompter: promptUtils.NewPrompt()},
		&RealAWSUtilityClient{GeneralManager: generalUtils.NewGeneralUtilsManager()},
	)
}
