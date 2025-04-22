package connection

import (
	"context"
	"errors"
	"testing"

	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestRealSSMStarter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSMClient := mock_awsctl.NewMockSSMClientInterface(ctrl)
	mockCommandExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	newTestSSMStarter := func() *RealSSMStarter {
		return &RealSSMStarter{
			client:          mockSSMClient,
			region:          "us-west-2",
			commandExecutor: mockCommandExecutor,
		}
	}

	t.Run("StartSession", func(t *testing.T) {
		tests := []struct {
			name          string
			setupMocks    func()
			expectError   bool
			errorContains string
		}{
			{
				name: "successful session start",
				setupMocks: func() {
					mockSSMClient.EXPECT().StartSession(gomock.Any(), gomock.Any()).Return(&ssm.StartSessionOutput{
						SessionId:  aws.String("test-session-id"),
						StreamUrl:  aws.String("wss://test-stream"),
						TokenValue: aws.String("test-token"),
					}, nil)
					mockCommandExecutor.EXPECT().LookPath(gomock.Any()).Return("/path/to/plugin", nil)
					mockCommandExecutor.EXPECT().RunInteractiveCommand(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					mockSSMClient.EXPECT().TerminateSession(gomock.Any(), gomock.Any()).Return(nil, nil)
				},
			},
			{
				name: "session start fails",
				setupMocks: func() {
					mockSSMClient.EXPECT().StartSession(gomock.Any(), gomock.Any()).Return(nil, errors.New("start error"))
				},
				expectError:   true,
				errorContains: "SSM session failed",
			},
			{
				name: "plugin not found",
				setupMocks: func() {
					mockSSMClient.EXPECT().StartSession(gomock.Any(), gomock.Any()).Return(&ssm.StartSessionOutput{
						SessionId: aws.String("test-session-id"),
					}, nil)
					mockCommandExecutor.EXPECT().LookPath(gomock.Any()).Return("", errors.New("not found"))
					mockSSMClient.EXPECT().TerminateSession(gomock.Any(), gomock.Any()).Return(nil, nil)
				},
				expectError:   true,
				errorContains: "session-manager-plugin not found",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.setupMocks()
				s := newTestSSMStarter()
				err := s.StartSession(context.Background(), "i-1234567890")

				if tt.expectError {
					assert.Error(t, err)
					if tt.errorContains != "" {
						assert.Contains(t, err.Error(), tt.errorContains)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("StartPortForwarding", func(t *testing.T) {
		tests := []struct {
			name          string
			setupMocks    func()
			expectError   bool
			errorContains string
		}{
			{
				name: "successful port forwarding",
				setupMocks: func() {
					mockSSMClient.EXPECT().StartSession(gomock.Any(), &ssm.StartSessionInput{
						Target:       aws.String("i-1234567890"),
						DocumentName: aws.String("AWS-StartPortForwardingSessionToRemoteHost"),
						Parameters: map[string][]string{
							"portNumber":      {"8080"},
							"localPortNumber": {"8080"},
							"host":            {"test.com"},
						},
					}).Return(&ssm.StartSessionOutput{
						SessionId:  aws.String("test-session-id"),
						StreamUrl:  aws.String("wss://test-stream"),
						TokenValue: aws.String("test-token"),
					}, nil)
					mockCommandExecutor.EXPECT().LookPath(gomock.Any()).Return("/path/to/plugin", nil)
					mockCommandExecutor.EXPECT().RunInteractiveCommand(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					mockSSMClient.EXPECT().TerminateSession(gomock.Any(), gomock.Any()).Return(nil, nil)
				},
			},
			{
				name: "port forwarding fails",
				setupMocks: func() {
					mockSSMClient.EXPECT().StartSession(gomock.Any(), gomock.Any()).Return(nil, errors.New("forwarding error"))
				},
				expectError:   true,
				errorContains: "SSM port forwarding failed",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.setupMocks()
				s := newTestSSMStarter()
				err := s.StartPortForwarding(context.Background(), "i-1234567890", 8080, "test.com", 8080)

				if tt.expectError {
					assert.Error(t, err)
					if tt.errorContains != "" {
						assert.Contains(t, err.Error(), tt.errorContains)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("StartSOCKSProxy", func(t *testing.T) {
		tests := []struct {
			name          string
			setupMocks    func()
			expectError   bool
			errorContains string
		}{
			{
				name: "successful SOCKS proxy",
				setupMocks: func() {
					mockSSMClient.EXPECT().StartSession(gomock.Any(), &ssm.StartSessionInput{
						Target:       aws.String("i-1234567890"),
						DocumentName: aws.String("AWS-StartPortForwardingSession"),
						Parameters: map[string][]string{
							"portNumber":      {"1080"},
							"localPortNumber": {"8080"},
						},
					}).Return(&ssm.StartSessionOutput{
						SessionId:  aws.String("test-session-id"),
						StreamUrl:  aws.String("wss://test-stream"),
						TokenValue: aws.String("test-token"),
					}, nil)
					mockCommandExecutor.EXPECT().LookPath(gomock.Any()).Return("/path/to/plugin", nil)
					mockCommandExecutor.EXPECT().RunInteractiveCommand(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					mockSSMClient.EXPECT().TerminateSession(gomock.Any(), gomock.Any()).Return(nil, nil)
				},
			},
			{
				name: "SOCKS proxy fails",
				setupMocks: func() {
					mockSSMClient.EXPECT().StartSession(gomock.Any(), gomock.Any()).Return(nil, errors.New("proxy error"))
				},
				expectError:   true,
				errorContains: "SSM SOCKS proxy failed",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.setupMocks()
				s := newTestSSMStarter()
				err := s.StartSOCKSProxy(context.Background(), "i-1234567890", 8080)

				if tt.expectError {
					assert.Error(t, err)
					if tt.errorContains != "" {
						assert.Contains(t, err.Error(), tt.errorContains)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("TerminateSession", func(t *testing.T) {
		t.Run("successful termination", func(t *testing.T) {
			mockSSMClient.EXPECT().TerminateSession(gomock.Any(), &ssm.TerminateSessionInput{
				SessionId: aws.String("test-session"),
			}).Return(&ssm.TerminateSessionOutput{}, nil)

			s := newTestSSMStarter()
			s.terminateSession(context.Background(), aws.String("test-session"))
		})

		t.Run("nil session ID", func(t *testing.T) {
			s := newTestSSMStarter()
			s.terminateSession(context.Background(), nil)
		})

		t.Run("termination fails", func(t *testing.T) {
			mockSSMClient.EXPECT().TerminateSession(gomock.Any(), gomock.Any()).Return(nil, errors.New("terminate error"))

			s := newTestSSMStarter()
			s.terminateSession(context.Background(), aws.String("test-session"))
		})
	})

	t.Run("runSessionManagerPlugin", func(t *testing.T) {
		tests := []struct {
			name          string
			setupMocks    func()
			expectError   bool
			errorContains string
		}{
			{
				name: "plugin execution success",
				setupMocks: func() {
					mockCommandExecutor.EXPECT().LookPath(gomock.Any()).Return("/path/to/plugin", nil)
					mockCommandExecutor.EXPECT().RunInteractiveCommand(gomock.Any(), "/path/to/plugin", gomock.Any()).Return(nil)
				},
			},
			{
				name: "plugin not found",
				setupMocks: func() {
					mockCommandExecutor.EXPECT().LookPath(gomock.Any()).Return("", errors.New("not found"))
				},
				expectError:   true,
				errorContains: "session-manager-plugin not found",
			},
			{
				name: "plugin execution fails",
				setupMocks: func() {
					mockCommandExecutor.EXPECT().LookPath(gomock.Any()).Return("/path/to/plugin", nil)
					mockCommandExecutor.EXPECT().RunInteractiveCommand(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("execution failed"))
				},
				expectError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.setupMocks()
				s := newTestSSMStarter()
				err := s.runSessionManagerPlugin(
					context.Background(),
					&ssm.StartSessionOutput{
						SessionId:  aws.String("test-session"),
						StreamUrl:  aws.String("wss://test"),
						TokenValue: aws.String("test-token"),
					},
					"i-1234567890",
					"test",
				)

				if tt.expectError {
					assert.Error(t, err)
					if tt.errorContains != "" {
						assert.Contains(t, err.Error(), tt.errorContains)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}
