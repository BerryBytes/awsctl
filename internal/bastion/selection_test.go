package bastion

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewBastionPrompter(t *testing.T) {
	prompter := NewBastionPrompter()
	assert.NotNil(t, prompter)
	assert.NotNil(t, prompter.Prompter)
}

func TestBastionPrompter_SelectAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	bp := &BastionPrompter{Prompter: mockPrompter}

	tests := []struct {
		name           string
		mockReturn     []interface{}
		expectedResult string
		expectedError  error
	}{
		{
			name: "successful selection",
			mockReturn: []interface{}{
				SSHIntoBastion, nil,
			},
			expectedResult: SSHIntoBastion,
			expectedError:  nil,
		},
		{
			name: "interrupted",
			mockReturn: []interface{}{
				"", promptUtils.ErrInterrupted,
			},
			expectedResult: "",
			expectedError:  promptUtils.ErrInterrupted,
		},
		{
			name: "other error",
			mockReturn: []interface{}{
				"", fmt.Errorf("some error"),
			},
			expectedResult: "",
			expectedError:  fmt.Errorf("failed to select action: some error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrompter.EXPECT().PromptForSelection(
				"What would you like to do?",
				[]string{SSHIntoBastion, StartSOCKSProxy, PortForwarding, ExitBastion},
			).Return(tt.mockReturn[0], tt.mockReturn[1])

			result, err := bp.SelectAction()

			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBastionPrompter_PromptForSOCKSProxyPort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)

	bp := &BastionPrompter{Prompter: mockPrompter}

	tests := []struct {
		name           string
		defaultPort    int
		mockReturn     []interface{}
		expectedResult int
		expectedError  error
	}{
		{
			name:        "successful input",
			defaultPort: 9999,
			mockReturn: []interface{}{
				"9999", nil,
			},
			expectedResult: 9999,
			expectedError:  nil,
		},
		{
			name:        "invalid port number",
			defaultPort: 9999,
			mockReturn: []interface{}{
				"not-a-number", nil,
			},
			expectedResult: 0,
			expectedError:  fmt.Errorf("invalid port number: strconv.Atoi: parsing \"not-a-number\": invalid syntax"),
		},
		{
			name:        "port out of range",
			defaultPort: 9999,
			mockReturn: []interface{}{
				"70000", nil,
			},
			expectedResult: 0,
			expectedError:  errors.New("port must be between 1 and 65535"),
		},
		{
			name:        "interrupted",
			defaultPort: 9999,
			mockReturn: []interface{}{
				"", promptUtils.ErrInterrupted,
			},
			expectedResult: 0,
			expectedError:  promptUtils.ErrInterrupted,
		},
		{
			name:        "generic error",
			defaultPort: 9999,
			mockReturn: []interface{}{
				"", fmt.Errorf("input error"),
			},
			expectedResult: 0,
			expectedError:  fmt.Errorf("failed to get SOCKS proxy port: input error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := fmt.Sprintf("Enter SOCKS proxy port (default: %d)", tt.defaultPort)
			mockPrompter.EXPECT().PromptForInput(prompt, strconv.Itoa(tt.defaultPort)).Return(tt.mockReturn[0], tt.mockReturn[1])

			result, err := bp.PromptForSOCKSProxyPort(tt.defaultPort)

			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBastionPrompter_PromptForBastionHost(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)

	bp := &BastionPrompter{Prompter: mockPrompter}

	tests := []struct {
		name           string
		mockReturn     []interface{}
		expectedResult string
		expectedError  error
	}{
		{
			name: "successful input",
			mockReturn: []interface{}{
				"bastion.example.com", nil,
			},
			expectedResult: "bastion.example.com",
			expectedError:  nil,
		},
		{
			name: "empty input",
			mockReturn: []interface{}{
				"", nil,
			},
			expectedResult: "",
			expectedError:  errors.New("bastion host cannot be empty"),
		},
		{
			name: "interrupted",
			mockReturn: []interface{}{
				"", promptUtils.ErrInterrupted,
			},
			expectedResult: "",
			expectedError:  promptUtils.ErrInterrupted,
		},
		{
			name: "generic error",
			mockReturn: []interface{}{
				"", fmt.Errorf("input error"),
			},
			expectedResult: "",
			expectedError:  fmt.Errorf("failed to get bastion host: input error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrompter.EXPECT().PromptForInput("Enter bastion host IP or DNS name:", "").Return(tt.mockReturn[0], tt.mockReturn[1])

			result, err := bp.PromptForBastionHost()

			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBastionPrompter_PromptForSSHUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)

	bp := &BastionPrompter{Prompter: mockPrompter}

	tests := []struct {
		name           string
		defaultUser    string
		mockReturn     []interface{}
		expectedResult string
		expectedError  error
	}{
		{
			name:        "successful input",
			defaultUser: "ubuntu",
			mockReturn: []interface{}{
				"ubuntu", nil,
			},
			expectedResult: "ubuntu",
			expectedError:  nil,
		},
		{
			name:        "custom input",
			defaultUser: "ubuntu",
			mockReturn: []interface{}{
				"ec2-user", nil,
			},
			expectedResult: "ec2-user",
			expectedError:  nil,
		},
		{
			name:        "interrupted",
			defaultUser: "ubuntu",
			mockReturn: []interface{}{
				"", promptUtils.ErrInterrupted,
			},
			expectedResult: "",
			expectedError:  promptUtils.ErrInterrupted,
		},
		{
			name:        "generic error",
			defaultUser: "ubuntu",
			mockReturn: []interface{}{
				"", fmt.Errorf("input error"),
			},
			expectedResult: "",
			expectedError:  fmt.Errorf("failed to get SSH user: input error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := fmt.Sprintf("Enter SSH user (default: %s)", tt.defaultUser)
			mockPrompter.EXPECT().PromptForInput(prompt, tt.defaultUser).Return(tt.mockReturn[0], tt.mockReturn[1])

			result, err := bp.PromptForSSHUser(tt.defaultUser)

			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBastionPrompter_PromptForLocalPort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)

	bp := &BastionPrompter{Prompter: mockPrompter}

	tests := []struct {
		name           string
		service        string
		defaultPort    int
		mockReturn     []interface{}
		expectedResult int
		expectedError  error
	}{
		{
			name:        "successful input",
			service:     "port forwarding",
			defaultPort: 3500,
			mockReturn: []interface{}{
				"3500", nil,
			},
			expectedResult: 3500,
			expectedError:  nil,
		},
		{
			name:        "invalid port",
			service:     "port forwarding",
			defaultPort: 3500,
			mockReturn: []interface{}{
				"invalid", nil,
			},
			expectedResult: 0,
			expectedError:  fmt.Errorf("invalid port number"),
		},
		{
			name:           "invalid default port",
			service:        "port forwarding",
			defaultPort:    70000,
			mockReturn:     []interface{}{},
			expectedResult: 0,
			expectedError:  fmt.Errorf("invalid default port number"),
		},
		{
			name:        "interrupted",
			service:     "port forwarding",
			defaultPort: 3500,
			mockReturn: []interface{}{
				"", promptUtils.ErrInterrupted,
			},
			expectedResult: 0,
			expectedError:  promptUtils.ErrInterrupted,
		},
		{
			name:        "generic error",
			service:     "port forwarding",
			defaultPort: 3500,
			mockReturn: []interface{}{
				"", fmt.Errorf("input error"),
			},
			expectedResult: 0,
			expectedError:  fmt.Errorf("failed to get local port: input error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := fmt.Sprintf("Enter local port for %s [default: %d]:", tt.service, tt.defaultPort)
			if tt.defaultPort < 1 || tt.defaultPort > 65535 {
			} else {
				mockPrompter.EXPECT().PromptForInput(prompt, strconv.Itoa(tt.defaultPort)).Return(tt.mockReturn[0], tt.mockReturn[1])
			}

			result, err := bp.PromptForLocalPort(tt.service, tt.defaultPort)

			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
func TestBastionPrompter_PromptForLocalPort_PortInUse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	bp := &BastionPrompter{Prompter: mockPrompter}

	listener, err := net.Listen("tcp", ":3500")
	assert.NoError(t, err)
	defer listener.Close()

	mockPrompter.EXPECT().PromptForInput(
		"Enter local port for port forwarding [default: 3500]:",
		"3500",
	).Return("3500", nil)

	result, err := bp.PromptForLocalPort("port forwarding", 3500)
	assert.NoError(t, err)
	assert.NotEqual(t, 3500, result)
}

func TestBastionPrompter_PromptForRemoteHost(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)

	bp := &BastionPrompter{Prompter: mockPrompter}

	tests := []struct {
		name           string
		mockReturn     []interface{}
		expectedResult string
		expectedError  error
	}{
		{
			name: "successful input",
			mockReturn: []interface{}{
				"db.example.com", nil,
			},
			expectedResult: "db.example.com",
			expectedError:  nil,
		},
		{
			name: "empty input",
			mockReturn: []interface{}{
				"", nil,
			},
			expectedResult: "",
			expectedError:  errors.New("remote host cannot be empty"),
		},
		{
			name: "interrupted",
			mockReturn: []interface{}{
				"", promptUtils.ErrInterrupted,
			},
			expectedResult: "",
			expectedError:  promptUtils.ErrInterrupted,
		},
		{
			name: "generic error",
			mockReturn: []interface{}{
				"", fmt.Errorf("input error"),
			},
			expectedResult: "",
			expectedError:  fmt.Errorf("failed to get remote host: input error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrompter.EXPECT().PromptForInput("Enter remote host IP or DNS name:", "").Return(tt.mockReturn[0], tt.mockReturn[1])

			result, err := bp.PromptForRemoteHost()

			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBastionPrompter_PromptForRemotePort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)

	bp := &BastionPrompter{Prompter: mockPrompter}

	tests := []struct {
		name           string
		service        string
		mockReturn     []interface{}
		expectedResult int
		expectedError  error
	}{
		{
			name:    "successful input",
			service: "remote",
			mockReturn: []interface{}{
				"5432", nil,
			},
			expectedResult: 5432,
			expectedError:  nil,
		},
		{
			name:    "invalid port",
			service: "remote",
			mockReturn: []interface{}{
				"invalid", nil,
			},
			expectedResult: 0,
			expectedError:  fmt.Errorf("invalid port number: strconv.Atoi: parsing \"invalid\": invalid syntax"),
		},
		{
			name:    "interrupted",
			service: "remote",
			mockReturn: []interface{}{
				"", promptUtils.ErrInterrupted,
			},
			expectedResult: 0,
			expectedError:  promptUtils.ErrInterrupted,
		},
		{
			name:    "generic error",
			service: "remote",
			mockReturn: []interface{}{
				"", fmt.Errorf("input error"),
			},
			expectedResult: 0,
			expectedError:  fmt.Errorf("failed to get remote port: input error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := fmt.Sprintf("Enter remote %s port", tt.service)
			mockPrompter.EXPECT().PromptForInput(prompt, "").Return(tt.mockReturn[0], tt.mockReturn[1])

			result, err := bp.PromptForRemotePort(tt.service)

			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBastionPrompter_PromptForSSHKeyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)

	bp := &BastionPrompter{Prompter: mockPrompter}

	tests := []struct {
		name           string
		defaultPath    string
		mockReturn     []interface{}
		expectedResult string
		expectedError  error
	}{
		{
			name:        "successful input",
			defaultPath: "~/.ssh/id_ed25519",
			mockReturn: []interface{}{
				"~/.ssh/id_ed25519", nil,
			},
			expectedResult: "~/.ssh/id_ed25519",
			expectedError:  nil,
		},
		{
			name:        "custom input",
			defaultPath: "~/.ssh/id_ed25519",
			mockReturn: []interface{}{
				"/custom/path/key.pem", nil,
			},
			expectedResult: "/custom/path/key.pem",
			expectedError:  nil,
		},
		{
			name:        "interrupted",
			defaultPath: "~/.ssh/id_ed25519",
			mockReturn: []interface{}{
				"", promptUtils.ErrInterrupted,
			},
			expectedResult: "",
			expectedError:  promptUtils.ErrInterrupted,
		},
		{
			name:        "generic error",
			defaultPath: "~/.ssh/id_ed25519",
			mockReturn: []interface{}{
				"", fmt.Errorf("input error"),
			},
			expectedResult: "",
			expectedError:  fmt.Errorf("failed to get SSH key path: input error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := fmt.Sprintf("Enter SSH key path (default: %s)", tt.defaultPath)
			mockPrompter.EXPECT().PromptForInput(prompt, tt.defaultPath).Return(tt.mockReturn[0], tt.mockReturn[1])

			result, err := bp.PromptForSSHKeyPath(tt.defaultPath)

			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBastionPrompter_PromptForBastionInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	bp := &BastionPrompter{Prompter: mockPrompter}

	instances := []models.EC2Instance{
		{
			InstanceID:      "i-123",
			PublicIPAddress: "1.2.3.4",
			Name:            "bastion-1",
		},
		{
			InstanceID:      "i-456",
			PublicIPAddress: "5.6.7.8",
			Name:            "",
		},
		{
			InstanceID:      "i-789",
			PublicIPAddress: "",
			Name:            "no-ip-bastion",
		},
	}

	tests := []struct {
		name           string
		mockReturns    [][]interface{}
		expectedResult string
		expectedError  error
	}{
		{
			name: "successful selection with name using public IP",
			mockReturns: [][]interface{}{
				{"Public IP (direct SSH)", nil},
				{"bastion-1 (i-123) - 1.2.3.4", nil},
			},
			expectedResult: "1.2.3.4",
			expectedError:  nil,
		},
		{
			name: "successful selection with name using instance ID",
			mockReturns: [][]interface{}{
				{"Instance ID (EC2 Instance Connect)", nil},
				{"bastion-1 (i-123) - 1.2.3.4", nil},
			},
			expectedResult: "i-123",
			expectedError:  nil,
		},
		{
			name: "successful selection without name",
			mockReturns: [][]interface{}{
				{"Public IP (direct SSH)", nil},
				{"i-456 (i-456) - 5.6.7.8", nil},
			},
			expectedResult: "5.6.7.8",
			expectedError:  nil,
		},
		{
			name:           "no instances",
			mockReturns:    [][]interface{}{},
			expectedResult: "",
			expectedError:  errors.New("no instances available"),
		},
		{
			name: "no public IP when public IP method selected",
			mockReturns: [][]interface{}{
				{"Public IP (direct SSH)", nil},
				{"no-ip-bastion (i-789) - ", nil},
			},
			expectedResult: "",
			expectedError:  errors.New("selected instance has no public IP"),
		},
		{
			name: "instance ID method with no public IP",
			mockReturns: [][]interface{}{
				{"Instance ID (EC2 Instance Connect)", nil},
				{"no-ip-bastion (i-789) - ", nil},
			},
			expectedResult: "i-789",
			expectedError:  nil,
		},
		{
			name: "interrupted at connection method selection",
			mockReturns: [][]interface{}{
				{"", promptUtils.ErrInterrupted},
			},
			expectedResult: "",
			expectedError:  promptUtils.ErrInterrupted,
		},
		{
			name: "interrupted at instance selection",
			mockReturns: [][]interface{}{
				{"Public IP (direct SSH)", nil},
				{"", promptUtils.ErrInterrupted},
			},
			expectedResult: "",
			expectedError:  promptUtils.ErrInterrupted,
		},
		{
			name: "generic error at connection method selection",
			mockReturns: [][]interface{}{
				{"", fmt.Errorf("selection error")},
			},
			expectedResult: "",
			expectedError:  fmt.Errorf("failed to select connection method: selection error"),
		},
		{
			name: "generic error at instance selection",
			mockReturns: [][]interface{}{
				{"Public IP (direct SSH)", nil},
				{"", fmt.Errorf("selection error")},
			},
			expectedResult: "",
			expectedError:  fmt.Errorf("failed to select bastion host: selection error"),
		},
		{
			name: "invalid instance selection",
			mockReturns: [][]interface{}{
				{"Public IP (direct SSH)", nil},
				{"invalid-item", nil},
			},
			expectedResult: "",
			expectedError:  errors.New("invalid selection"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "no instances" {
				_, err := bp.PromptForBastionInstance([]models.EC2Instance{})
				assert.EqualError(t, err, tt.expectedError.Error())
				return
			}

			connectionMethods := []string{"Public IP (direct SSH)", "Instance ID (EC2 Instance Connect)"}
			instanceItems := []string{
				"bastion-1 (i-123) - 1.2.3.4",
				"i-456 (i-456) - 5.6.7.8",
				"no-ip-bastion (i-789) - ",
			}

			if len(tt.mockReturns) > 0 {
				mockPrompter.EXPECT().PromptForSelection(
					"Select connection method:",
					connectionMethods,
				).Return(tt.mockReturns[0][0], tt.mockReturns[0][1])
			}

			if len(tt.mockReturns) > 1 && tt.mockReturns[0][1] == nil {
				mockPrompter.EXPECT().PromptForSelection(
					"Select bastion instance:",
					instanceItems,
				).Return(tt.mockReturns[1][0], tt.mockReturns[1][1])
			}

			result, err := bp.PromptForBastionInstance(instances)

			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPromptForConfirmation(t *testing.T) {
	tests := []struct {
		name          string
		mockInput     string
		expected      bool
		expectedError error
	}{
		{
			name:          "User enters 'y' for yes",
			mockInput:     "y",
			expected:      true,
			expectedError: nil,
		},
		{
			name:          "User enters 'yes' for yes",
			mockInput:     "yes",
			expected:      true,
			expectedError: nil,
		},
		{
			name:          "User enters 'n' for no",
			mockInput:     "n",
			expected:      false,
			expectedError: nil,
		},
		{
			name:          "User enters 'no' for no",
			mockInput:     "no",
			expected:      false,
			expectedError: nil,
		},
		{
			name:          "User presses Enter (defaults to no)",
			mockInput:     "",
			expected:      false,
			expectedError: nil,
		},
		{
			name:          "Prompt interrupted",
			mockInput:     "",
			expected:      false,
			expectedError: promptUtils.ErrInterrupted,
		},
		{
			name:          "Invalid input followed by valid 'y'",
			mockInput:     "invalid\ny",
			expected:      true,
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mock_awsctl.NewMockPrompter(ctrl)

			inputs := strings.Split(tt.mockInput, "\n")

			for _, input := range inputs {
				if tt.expectedError != nil && input == "" {
					mockPrompter.EXPECT().
						PromptForInput("Proceed with action? (y/N)", "n").
						Return("", promptUtils.ErrInterrupted)
				} else {
					mockPrompter.EXPECT().
						PromptForInput("Proceed with action? (y/N)", "n").
						Return(input, nil)
				}
			}

			bpTest := &BastionPrompter{
				Prompter: mockPrompter,
			}

			result, err := bpTest.PromptForConfirmation("Proceed with action?")

			assert.Equal(t, tt.expected, result)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
