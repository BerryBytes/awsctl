package bastion

import (
	"context"
	"errors"
	"testing"

	connection "github.com/BerryBytes/awsctl/internal/common"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestBastionService_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockServices := mock_awsctl.NewMockServicesInterface(ctrl)

	tests := []struct {
		name          string
		setupMocks    func()
		expectedError string
	}{
		{
			name: "SelectAction returns error",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return("", errors.New("prompt error"))
			},
			expectedError: "failed to select action: prompt error",
		},
		{
			name: "User selects ExitBastion",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.ExitBastion, nil)
			},
		},
		{
			name: "User selects SSHIntoBastion successfully",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.SSHIntoBastion, nil)
				mockServices.EXPECT().SSHIntoBastion(gomock.Any()).Return(nil)
			},
		},
		{
			name: "SSHIntoBastion returns error",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.SSHIntoBastion, nil)
				mockServices.EXPECT().SSHIntoBastion(gomock.Any()).Return(errors.New("ssh error"))
			},
			expectedError: "SSH/SSM failed: ssh error",
		},
		{
			name: "User selects StartSOCKSProxy successfully",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.StartSOCKSProxy, nil)
				mockPrompter.EXPECT().PromptForSOCKSProxyPort(1080).Return(1234, nil)
				mockServices.EXPECT().StartSOCKSProxy(gomock.Any(), 1234).Return(nil)
			},
		},
		{
			name: "PromptForSOCKSProxyPort returns error",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.StartSOCKSProxy, nil)
				mockPrompter.EXPECT().PromptForSOCKSProxyPort(1080).Return(0, errors.New("port error"))
			},
			expectedError: "failed to get port: port error",
		},
		{
			name: "StartSOCKSProxy returns error",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.StartSOCKSProxy, nil)
				mockPrompter.EXPECT().PromptForSOCKSProxyPort(1080).Return(1234, nil)
				mockServices.EXPECT().StartSOCKSProxy(gomock.Any(), 1234).Return(errors.New("proxy error"))
			},
			expectedError: "SOCKS proxy error: proxy error",
		},
		{
			name: "User selects PortForwarding successfully",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.PortForwarding, nil)
				mockPrompter.EXPECT().PromptForLocalPort("forwarding", 8080).Return(9000, nil)
				mockPrompter.EXPECT().PromptForRemoteHost().Return("remote.host", nil)
				mockPrompter.EXPECT().PromptForRemotePort("remote service").Return(8081, nil)
				mockServices.EXPECT().StartPortForwarding(gomock.Any(), 9000, "remote.host", 8081).Return(nil)
			},
		},
		{
			name: "PortForwarding - PromptForLocalPort error",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.PortForwarding, nil)
				mockPrompter.EXPECT().PromptForLocalPort("forwarding", 8080).Return(0, errors.New("local port error"))
			},
			expectedError: "failed to get local port: local port error",
		},
		{
			name: "PortForwarding - PromptForRemoteHost error",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.PortForwarding, nil)
				mockPrompter.EXPECT().PromptForLocalPort("forwarding", 8080).Return(9000, nil)
				mockPrompter.EXPECT().PromptForRemoteHost().Return("", errors.New("host error"))
			},
			expectedError: "failed to get remote host: host error",
		},
		{
			name: "PortForwarding - PromptForRemotePort error",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.PortForwarding, nil)
				mockPrompter.EXPECT().PromptForLocalPort("forwarding", 8080).Return(9000, nil)
				mockPrompter.EXPECT().PromptForRemoteHost().Return("remote.host", nil)
				mockPrompter.EXPECT().PromptForRemotePort("remote service").Return(0, errors.New("remote port error"))
			},
			expectedError: "failed to get remote port: remote port error",
		},
		{
			name: "PortForwarding - StartPortForwarding error",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.PortForwarding, nil)
				mockPrompter.EXPECT().PromptForLocalPort("forwarding", 8080).Return(9000, nil)
				mockPrompter.EXPECT().PromptForRemoteHost().Return("remote.host", nil)
				mockPrompter.EXPECT().PromptForRemotePort("remote service").Return(8081, nil)
				mockServices.EXPECT().StartPortForwarding(gomock.Any(), 9000, "remote.host", 8081).Return(errors.New("forwarding error"))
			},
			expectedError: "port forwarding error: forwarding error",
		},
		{
			name: "Unknown action",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return("unknown-action", nil)
			},
			expectedError: "unknown action: unknown-action",
		},
		{
			name: "User interruption in SelectAction",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return("", promptUtils.ErrInterrupted)
			},
		},
		{
			name: "User interruption in SSHIntoBastion",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.SSHIntoBastion, nil)
				mockServices.EXPECT().SSHIntoBastion(gomock.Any()).Return(promptUtils.ErrInterrupted)
			},
		},
		{
			name: "User interruption in SOCKS proxy port prompt",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.StartSOCKSProxy, nil)
				mockPrompter.EXPECT().PromptForSOCKSProxyPort(1080).Return(0, promptUtils.ErrInterrupted)
			},
		},
		{
			name: "User interruption in PortForwarding local port prompt",
			setupMocks: func() {
				mockPrompter.EXPECT().SelectAction().Return(connection.PortForwarding, nil)
				mockPrompter.EXPECT().PromptForLocalPort("forwarding", 8080).Return(0, promptUtils.ErrInterrupted)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()

			service := NewBastionService(mockServices, mockPrompter)
			err := service.Run(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
