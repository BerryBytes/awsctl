package sso

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock AWSSSOClient for testing

type MockAWSSSOClient struct {
	mock.Mock
}

func (m *MockAWSSSOClient) GetCachedSsoAccessToken(profile string) (string, error) {
	args := m.Called(profile)
	return args.String(0), args.Error(1)
}

func (m *MockAWSSSOClient) ConfigureSSO() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAWSSSOClient) GetSSOProfiles() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAWSSSOClient) GetSSOAccountName(accountID, profile string) (string, error) {
	args := m.Called(accountID, profile)
	return args.String(0), args.Error(1)
}

func (m *MockAWSSSOClient) GetSSORoles(profile, accountID string) ([]string, error) {
	args := m.Called(profile, accountID)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAWSSSOClient) SSOLogin(awsProfile string, refresh, noBrowser bool) error {
	args := m.Called(awsProfile, refresh, noBrowser)
	return args.Error(0)
}

func TestSSOLogin_Success(t *testing.T) {
	mockClient := new(MockAWSSSOClient)
	mockClient.On("SSOLogin", "test-profile", false, false).Return(nil)

	err := mockClient.SSOLogin("test-profile", false, false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestSSOLogin_Failure(t *testing.T) {
	mockClient := new(MockAWSSSOClient)
	mockClient.On("SSOLogin", "test-profile", false, false).Return(errors.New("SSO login failed"))

	err := mockClient.SSOLogin("test-profile", false, false)
	assert.Error(t, err)
	assert.Equal(t, "SSO login failed", err.Error())
	mockClient.AssertExpectations(t)
}

func TestGetCachedSsoAccessToken_Success(t *testing.T) {
	mockClient := new(MockAWSSSOClient)
	mockClient.On("GetCachedSsoAccessToken", "test-profile").Return("fake-token", nil)

	token, err := mockClient.GetCachedSsoAccessToken("test-profile")
	assert.NoError(t, err)
	assert.Equal(t, "fake-token", token)
	mockClient.AssertExpectations(t)
}

func TestGetCachedSsoAccessToken_Failure(t *testing.T) {
	mockClient := new(MockAWSSSOClient)
	mockClient.On("GetCachedSsoAccessToken", "test-profile").Return("", errors.New("failed to retrieve token"))

	token, err := mockClient.GetCachedSsoAccessToken("test-profile")
	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Equal(t, "failed to retrieve token", err.Error())
	mockClient.AssertExpectations(t)
}

func TestGetSSOProfiles(t *testing.T) {
	mockClient := new(MockAWSSSOClient)
	mockClient.On("GetSSOProfiles").Return([]string{"profile1", "profile2"}, nil)

	profiles, err := mockClient.GetSSOProfiles()
	assert.NoError(t, err)
	assert.Equal(t, []string{"profile1", "profile2"}, profiles)
	mockClient.AssertExpectations(t)
}

func TestGetSSOAccountName(t *testing.T) {
	mockClient := new(MockAWSSSOClient)
	mockClient.On("GetSSOAccountName", "123456789012", "test-profile").Return("TestAccount", nil)

	accountName, err := mockClient.GetSSOAccountName("123456789012", "test-profile")
	assert.NoError(t, err)
	assert.Equal(t, "TestAccount", accountName)
	mockClient.AssertExpectations(t)
}

func TestGetSSORoles(t *testing.T) {
	mockClient := new(MockAWSSSOClient)
	mockClient.On("GetSSORoles", "test-profile", "123456789012").Return([]string{"Admin", "ReadOnly"}, nil)

	roles, err := mockClient.GetSSORoles("test-profile", "123456789012")
	assert.NoError(t, err)
	assert.Equal(t, []string{"Admin", "ReadOnly"}, roles)
	mockClient.AssertExpectations(t)
}
