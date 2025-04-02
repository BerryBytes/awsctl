package sso

import (
	"testing"

	mock_sso "github.com/BerryBytes/awsctl/tests/mocks"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestConfigureSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_sso.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "region", "us-west-2", "--profile", "test-profile").Return([]byte{}, nil).Times(1)

	client := &RealAWSConfigClient{Executor: mockExecutor}

	err := client.ConfigureSet("region", "us-west-2", "test-profile")
	assert.NoError(t, err)
}

func TestConfigureGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_sso.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "get", "region", "--profile", "test-profile").Return([]byte("us-west-2\n"), nil).Times(1)

	client := &RealAWSConfigClient{Executor: mockExecutor}

	region, err := client.ConfigureGet("region", "test-profile")
	assert.NoError(t, err)
	assert.Equal(t, "us-west-2", region)
}

func TestValidProfiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_sso.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "list-profiles").Return([]byte("test-profile\nanother-profile\n"), nil).Times(1)

	client := &RealAWSConfigClient{Executor: mockExecutor}

	profiles, err := client.ValidProfiles()
	assert.NoError(t, err)
	assert.Equal(t, []string{"test-profile", "another-profile"}, profiles)
}

func TestConfigureDefaultProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_sso.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "region", "us-west-2", "--profile", "default").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "output", "json", "--profile", "default").Return([]byte{}, nil).Times(1)

	client := &RealAWSConfigClient{Executor: mockExecutor}

	err := client.ConfigureDefaultProfile("us-west-2", "json")
	assert.NoError(t, err)
}

func TestConfigureSSOProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_sso.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_region", "us-west-2", "--profile", "test-profile").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_account_id", "123456789012", "--profile", "test-profile").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_start_url", "https://my-sso-url", "--profile", "test-profile").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_role_name", "my-role", "--profile", "test-profile").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "region", "us-west-2", "--profile", "test-profile").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "output", "json", "--profile", "test-profile").Return([]byte{}, nil).Times(1)

	client := &RealAWSConfigClient{Executor: mockExecutor}

	err := client.ConfigureSSOProfile("test-profile", "us-west-2", "123456789012", "my-role", "https://my-sso-url")
	assert.NoError(t, err)
}

func TestGetAWSRegion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_sso.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "get", "region", "--profile", "test-profile").Return([]byte("us-west-2\n"), nil).Times(1)

	client := &RealAWSConfigClient{Executor: mockExecutor}

	region, err := client.GetAWSRegion("test-profile")
	assert.NoError(t, err)
	assert.Equal(t, "us-west-2", region)
}

func TestGetAWSOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_sso.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "get", "output", "--profile", "test-profile").Return([]byte("json\n"), nil).Times(1)

	client := &RealAWSConfigClient{Executor: mockExecutor}

	output, err := client.GetAWSOutput("test-profile")
	assert.NoError(t, err)
	assert.Equal(t, "json", output)
}
