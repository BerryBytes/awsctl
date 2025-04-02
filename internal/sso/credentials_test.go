package sso_test

import (
	"testing"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/models"
	mock_sso "github.com/BerryBytes/awsctl/tests/mocks"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetRoleCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_sso.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_sso.NewMockAWSConfigClient(ctrl)

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

	mockExecutor := mock_sso.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_sso.NewMockAWSConfigClient(ctrl)

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

	mockExecutor := mock_sso.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_sso.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	mockExecutor.EXPECT().RunCommand("aws", "sts", "get-caller-identity", "--profile", "mockProfile").
		Return([]byte(`{"UserId":"mockUserID"}`), nil)

	isValid := client.IsCallerIdentityValid("mockProfile")

	assert.True(t, isValid)
}

func TestAwsSTSGetCallerIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_sso.NewMockCommandExecutor(ctrl)
	mockConfigClient := mock_sso.NewMockAWSConfigClient(ctrl)

	client := sso.NewRealAWSCredentialsClient(mockConfigClient, mockExecutor)

	mockExecutor.EXPECT().RunCommand("aws", "sts", "get-caller-identity", "--profile", "mockProfile").
		Return([]byte(`{"Arn":"arn:aws:sts::123456789012:assumed-role/mockRole/mockSession"}`), nil)

	arn, err := client.AwsSTSGetCallerIdentity("mockProfile")

	assert.NoError(t, err)
	assert.Equal(t, "arn:aws:sts::123456789012:assumed-role/mockRole/mockSession", arn)
}
