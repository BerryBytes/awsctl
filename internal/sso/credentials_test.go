package sso_test

import (
	"fmt"
	"testing"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetRoleCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	accessToken := "mockAccessToken"
	roleName := "mockRoleName"
	accountID := "mockAccountID"

	mockExecutor.EXPECT().RunCommand("aws", "sso", "get-role-credentials",
		"--access-token", accessToken,
		"--role-name", roleName,
		"--account-id", accountID).Return([]byte(`{"RoleCredentials":{"AccessKeyId":"AKIA...","SecretAccessKey":"secret...","SessionToken":"session_token","Expiration":1616198901000}}`), nil)

	creds, err := client.GetRoleCredentials(accessToken, roleName, accountID)

	assert.NoError(t, err)
	assert.NotNil(t, creds)
	assert.Equal(t, "AKIA...", creds.AccessKeyID)
	assert.Equal(t, "secret...", creds.SecretAccessKey)
	assert.Equal(t, "session_token", creds.SessionToken)
}

func TestSaveAWSCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	creds := &models.AWSCredentials{
		AccessKeyID:     "AKIA...",
		SecretAccessKey: "secret...",
		SessionToken:    "session_token",
	}

	mockConfigClient.EXPECT().ConfigureSet("aws_access_key_id", "AKIA...", "mockProfile").Return(nil)
	mockConfigClient.EXPECT().ConfigureSet("aws_secret_access_key", "secret...", "mockProfile").Return(nil)
	mockConfigClient.EXPECT().ConfigureSet("aws_session_token", "session_token", "mockProfile").Return(nil)

	err := client.SaveAWSCredentials("mockProfile", creds)

	assert.NoError(t, err)
}

func TestIsCallerIdentityValid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	mockExecutor.EXPECT().RunCommand("aws", "sts", "get-caller-identity", "--profile", "mockProfile").
		Return([]byte(`{"UserId":"mockUserID"}`), nil)

	isValid := client.IsCallerIdentityValid("mockProfile")

	assert.True(t, isValid)
}

func TestAwsSTSGetCallerIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	mockExecutor.EXPECT().RunCommand("aws", "sts", "get-caller-identity", "--profile", "mockProfile").
		Return([]byte(`{"Arn":"arn:aws:sts::123456789012:assumed-role/mockRole/mockSession"}`), nil)

	arn, err := client.AwsSTSGetCallerIdentity("mockProfile")

	assert.NoError(t, err)
	assert.Equal(t, "arn:aws:sts::123456789012:assumed-role/mockRole/mockSession", arn)
}

func TestGetRoleCredentials_ErrorHandling_RunCommandFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	accessToken := "mockAccessToken"
	roleName := "mockRoleName"
	accountID := "mockAccountID"

	mockExecutor.EXPECT().RunCommand("aws", "sso", "get-role-credentials",
		"--access-token", accessToken,
		"--role-name", roleName,
		"--account-id", accountID).Return([]byte(""), fmt.Errorf("command error"))

	creds, err := client.GetRoleCredentials(accessToken, roleName, accountID)

	assert.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "failed to get role credentials")
}

func TestGetRoleCredentials_ErrorHandling_JSONParseFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	accessToken := "mockAccessToken"
	roleName := "mockRoleName"
	accountID := "mockAccountID"

	mockExecutor.EXPECT().RunCommand("aws", "sso", "get-role-credentials",
		"--access-token", accessToken,
		"--role-name", roleName,
		"--account-id", accountID).Return([]byte(`{"invalidJson"`), nil)

	creds, err := client.GetRoleCredentials(accessToken, roleName, accountID)

	assert.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "failed to parse credentials JSON")
}

func TestSaveAWSCredentials_ErrorHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	creds := &models.AWSCredentials{
		AccessKeyID:     "AKIA...",
		SecretAccessKey: "secret...",
		SessionToken:    "session_token",
	}

	mockConfigClient.EXPECT().ConfigureSet("aws_access_key_id", "AKIA...", "mockProfile").Return(fmt.Errorf("config error"))

	err := client.SaveAWSCredentials("mockProfile", creds)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set aws_access_key_id for profile mockProfile")
}

func TestIsCallerIdentityValid_RunCommandFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	mockExecutor.EXPECT().RunCommand("aws", "sts", "get-caller-identity", "--profile", "mockProfile").Return(nil, fmt.Errorf("command error"))

	isValid := client.IsCallerIdentityValid("mockProfile")

	assert.False(t, isValid)
}

func TestAwsSTSGetCallerIdentity_RunCommandFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	mockExecutor.EXPECT().RunCommand("aws", "sts", "get-caller-identity", "--profile", "mockProfile").Return(nil, fmt.Errorf("command error"))

	arn, err := client.AwsSTSGetCallerIdentity("mockProfile")

	assert.Error(t, err)
	assert.Empty(t, arn)
	assert.Contains(t, err.Error(), "failed to get caller identity")
}

func TestAwsSTSGetCallerIdentity_JSONParseFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	mockExecutor.EXPECT().RunCommand("aws", "sts", "get-caller-identity", "--profile", "mockProfile").Return([]byte(`{"invalidJson"`), nil)

	arn, err := client.AwsSTSGetCallerIdentity("mockProfile")

	assert.Error(t, err)
	assert.Empty(t, arn)
	assert.Contains(t, err.Error(), "failed to parse identity JSON")
}
