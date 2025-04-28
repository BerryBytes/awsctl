package connection_test

import (
	"errors"
	"testing"

	connection "github.com/BerryBytes/awsctl/internal/common"
	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestConnectionPrompter(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter)
		run            func(p *connection.ConnectionPrompterStruct) (interface{}, error)
		wantResult     interface{}
		wantErr        bool
		wantErrMessage string
	}{
		{
			name: "NewConnectionPrompter creates instance",
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return connection.NewConnectionPrompter(), nil
			},
			wantResult: &connection.ConnectionPrompterStruct{},
			wantErr:    false,
		},

		{
			name: "SelectAction success",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("What would you like to do?", []string{
					connection.SSHIntoBastion, connection.StartSOCKSProxy, connection.PortForwarding, connection.ExitBastion,
				}).Return(connection.SSHIntoBastion, nil)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.SelectAction() },
			wantResult:     connection.SSHIntoBastion,
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "SelectAction interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("What would you like to do?", gomock.Any()).Return("", promptUtils.ErrInterrupted)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.SelectAction() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "SelectAction generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("What would you like to do?", gomock.Any()).Return("", errors.New("generic error"))
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.SelectAction() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "failed to select action: generic error",
		},

		{
			name: "PromptForSOCKSProxyPort success",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter SOCKS proxy port (default: 1080)", "1080").Return("8080", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForSOCKSProxyPort(1080)
			},
			wantResult:     8080,
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForSOCKSProxyPort interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter SOCKS proxy port (default: 1080)", "1080").Return("", promptUtils.ErrInterrupted)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForSOCKSProxyPort(1080)
			},
			wantResult:     0,
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "PromptForSOCKSProxyPort generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter SOCKS proxy port (default: 1080)", "1080").Return("", errors.New("input error"))
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForSOCKSProxyPort(1080)
			},
			wantResult:     0,
			wantErr:        true,
			wantErrMessage: "failed to get SOCKS proxy port: input error",
		},
		{
			name: "PromptForSOCKSProxyPort invalid port number",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter SOCKS proxy port (default: 1080)", "1080").Return("not-a-number", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForSOCKSProxyPort(1080)
			},
			wantResult:     0,
			wantErr:        true,
			wantErrMessage: "invalid port number",
		},
		{
			name: "PromptForSOCKSProxyPort out of range",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter SOCKS proxy port (default: 1080)", "1080").Return("70000", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForSOCKSProxyPort(1080)
			},
			wantResult:     0,
			wantErr:        true,
			wantErrMessage: "port must be between 1 and 65535",
		},

		{
			name: "PromptForBastionHost success",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter bastion host IP or DNS name:", "").Return("bastion.example.com", nil)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForBastionHost() },
			wantResult:     "bastion.example.com",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForBastionHost interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter bastion host IP or DNS name:", "").Return("", promptUtils.ErrInterrupted)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForBastionHost() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "PromptForBastionHost generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter bastion host IP or DNS name:", "").Return("", errors.New("input error"))
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForBastionHost() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "failed to get bastion host: input error",
		},

		{
			name: "PromptForSSHUser success",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter SSH user (default: ec2-user)", "ec2-user").Return("admin", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForSSHUser("ec2-user")
			},
			wantResult:     "admin",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForSSHUser interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter SSH user (default: ec2-user)", "ec2-user").Return("", promptUtils.ErrInterrupted)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForSSHUser("ec2-user")
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "PromptForSSHUser generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter SSH user (default: ec2-user)", "ec2-user").Return("", errors.New("input error"))
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForSSHUser("ec2-user")
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "failed to get SSH user: input error",
		},

		{
			name: "PromptForLocalPort success",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter local port for testing [default: 8080]:", "8080").Return("8080", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForLocalPort("testing", 8080)
			},
			wantResult:     8080,
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForLocalPort interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter local port for testing [default: 8080]:", "8080").Return("", promptUtils.ErrInterrupted)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForLocalPort("testing", 8080)
			},
			wantResult:     0,
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "PromptForLocalPort generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter local port for testing [default: 8080]:", "8080").Return("", errors.New("input error"))
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForLocalPort("testing", 8080)
			},
			wantResult:     0,
			wantErr:        true,
			wantErrMessage: "failed to get local port: input error",
		},

		{
			name:  "PromptForLocalPort invalid default",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForLocalPort("testing", 0)
			},
			wantResult:     0,
			wantErr:        true,
			wantErrMessage: "invalid default port number",
		},
		{
			name: "PromptForLocalPort invalid input",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter local port for testing [default: 8080]:", "8080").Return("invalid", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForLocalPort("testing", 8080)
			},
			wantResult:     0,
			wantErr:        true,
			wantErrMessage: "invalid port number",
		},

		{
			name: "PromptForRemoteHost success",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter remote host IP or DNS name:", "").Return("internal.example.com", nil)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForRemoteHost() },
			wantResult:     "internal.example.com",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForRemoteHost interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter remote host IP or DNS name:", "").Return("", promptUtils.ErrInterrupted)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForRemoteHost() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "PromptForRemoteHost generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter remote host IP or DNS name:", "").Return("", errors.New("input error"))
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForRemoteHost() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "failed to get remote host: input error",
		},
		{
			name: "PromptForRemoteHost empty",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter remote host IP or DNS name:", "").Return("", nil)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForRemoteHost() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "remote host cannot be empty",
		},

		{
			name: "PromptForRemotePort success",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter remote service port", "").Return("80", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForRemotePort("service")
			},
			wantResult:     80,
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForRemotePort interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter remote service port", "").Return("", promptUtils.ErrInterrupted)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForRemotePort("service")
			},
			wantResult:     0,
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "PromptForRemotePort generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter remote service port", "").Return("", errors.New("input error"))
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForRemotePort("service")
			},
			wantResult:     0,
			wantErr:        true,
			wantErrMessage: "failed to get remote port: input error",
		},
		{
			name: "PromptForRemotePort invalid input",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter remote service port", "").Return("invalid", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForRemotePort("service")
			},
			wantResult:     0,
			wantErr:        true,
			wantErrMessage: "invalid port number",
		},

		{
			name: "PromptForSSHKeyPath success",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter SSH key path (default: ~/.ssh/id_rsa)", "~/.ssh/id_rsa").Return("/custom/key", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForSSHKeyPath("~/.ssh/id_rsa")
			},
			wantResult:     "/custom/key",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForSSHKeyPath interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter SSH key path (default: ~/.ssh/id_rsa)", "~/.ssh/id_rsa").Return("", promptUtils.ErrInterrupted)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForSSHKeyPath("~/.ssh/id_rsa")
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "PromptForSSHKeyPath generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter SSH key path (default: ~/.ssh/id_rsa)", "~/.ssh/id_rsa").Return("", errors.New("input error"))
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForSSHKeyPath("~/.ssh/id_rsa")
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "failed to get SSH key path: input error",
		},

		{
			name: "PromptForBastionInstance success PublicIP",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("Public IP (direct SSH)", nil)
				m.EXPECT().PromptForSelection("Select bastion instance:", gomock.Any()).Return("bastion1 (i-123) - 1.2.3.4", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				instances := []models.EC2Instance{{Name: "bastion1", InstanceID: "i-123", PublicIPAddress: "1.2.3.4"}}
				return p.PromptForBastionInstance(instances, false)
			},
			wantResult:     "1.2.3.4",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForBastionInstance interrupted selection",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("Public IP (direct SSH)", nil)
				m.EXPECT().PromptForSelection("Select bastion instance:", gomock.Any()).Return("", promptUtils.ErrInterrupted)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				instances := []models.EC2Instance{{Name: "bastion1", InstanceID: "i-123", PublicIPAddress: "1.2.3.4"}}
				return p.PromptForBastionInstance(instances, false)
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "PromptForBastionInstance generic error method",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("", errors.New("method error"))
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				instances := []models.EC2Instance{{Name: "bastion1", InstanceID: "i-123", PublicIPAddress: "1.2.3.4"}}
				return p.PromptForBastionInstance(instances, false)
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "failed to select connection method: method error",
		},
		{
			name: "PromptForBastionInstance generic error selection",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("Public IP (direct SSH)", nil)
				m.EXPECT().PromptForSelection("Select bastion instance:", gomock.Any()).Return("", errors.New("selection error"))
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				instances := []models.EC2Instance{{Name: "bastion1", InstanceID: "i-123", PublicIPAddress: "1.2.3.4"}}
				return p.PromptForBastionInstance(instances, false)
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "failed to select bastion instance: selection error",
		},
		{
			name:  "PromptForBastionInstance no instances",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForBastionInstance([]models.EC2Instance{}, false)
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "no instances available",
		},
		{
			name: "PromptForBastionInstance no public IP",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("Public IP (direct SSH)", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				instances := []models.EC2Instance{{Name: "bastion1", InstanceID: "i-123", PublicIPAddress: ""}}
				return p.PromptForBastionInstance(instances, false)
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "no bastion instances with public IP available",
		},
		{
			name: "PromptForBastionInstance invalid selection",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("Public IP (direct SSH)", nil)
				m.EXPECT().PromptForSelection("Select bastion instance:", gomock.Any()).Return("invalid selection", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				instances := []models.EC2Instance{{Name: "bastion1", InstanceID: "i-123", PublicIPAddress: "1.2.3.4"}}
				return p.PromptForBastionInstance(instances, false)
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "invalid selection",
		},

		{
			name: "PromptForConfirmation yes",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Confirm? (y/N)", "n").Return("y", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForConfirmation("Confirm?")
			},
			wantResult:     true,
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForConfirmation no",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Confirm? (y/N)", "n").Return("n", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForConfirmation("Confirm?")
			},
			wantResult:     false,
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForConfirmation default no",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Confirm? (y/N)", "n").Return("", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForConfirmation("Confirm?")
			},
			wantResult:     false,
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForConfirmation interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Confirm? (y/N)", "n").Return("", promptUtils.ErrInterrupted)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForConfirmation("Confirm?")
			},
			wantResult:     false,
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "PromptForConfirmation generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Confirm? (y/N)", "n").Return("", errors.New("input error"))
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForConfirmation("Confirm?")
			},
			wantResult:     false,
			wantErr:        true,
			wantErrMessage: "confirmation failed: input error",
		},
		{
			name: "PromptForConfirmation invalid then yes",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				gomock.InOrder(
					m.EXPECT().PromptForInput("Confirm? (y/N)", "n").Return("invalid", nil),
					m.EXPECT().PromptForInput("Confirm? (y/N)", "n").Return("yes", nil),
				)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForConfirmation("Confirm?")
			},
			wantResult:     true,
			wantErr:        false,
			wantErrMessage: "",
		},

		{
			name: "PromptForInstanceID success",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter EC2 instance ID:", "").Return("i-123", nil)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForInstanceID() },
			wantResult:     "i-123",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForInstanceID interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter EC2 instance ID:", "").Return("", promptUtils.ErrInterrupted)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForInstanceID() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "PromptForInstanceID generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter EC2 instance ID:", "").Return("", errors.New("input error"))
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForInstanceID() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "failed to get instance ID: input error",
		},

		{
			name: "ChooseConnectionMethod success SSH",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("SSH", nil)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.ChooseConnectionMethod() },
			wantResult:     "SSH",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "ChooseConnectionMethod interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("", promptUtils.ErrInterrupted)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.ChooseConnectionMethod() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "ChooseConnectionMethod generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("", errors.New("selection error"))
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.ChooseConnectionMethod() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "failed to choose connection method: selection error",
		},
		{
			name: "ChooseConnectionMethod unexpected selection",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("invalid", nil)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.ChooseConnectionMethod() },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "unexpected selection: invalid",
		},
		{
			name: "PromptForRegion success with default",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter AWS region (Default: us-east-1):", "us-east-1").Return("us-east-1", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForRegion("us-east-1")
			},
			wantResult:     "us-east-1",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForRegion success with custom input",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter AWS region (Default: us-east-1):", "us-east-1").Return("eu-west-1", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForRegion("us-east-1")
			},
			wantResult:     "eu-west-1",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForRegion success no default",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter AWS region:", "").Return("us-west-2", nil)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForRegion("") },
			wantResult:     "us-west-2",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForRegion interrupted",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter AWS region (Default: us-east-1):", "us-east-1").Return("", promptUtils.ErrInterrupted)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForRegion("us-east-1")
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: promptUtils.ErrInterrupted.Error(),
		},
		{
			name: "PromptForRegion generic error",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter AWS region (Default: us-east-1):", "us-east-1").Return("", errors.New("input error"))
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				return p.PromptForRegion("us-east-1")
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "failed to get region: input error",
		},
		{
			name: "PromptForRegion empty input",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForInput("Enter AWS region:", "").Return("", nil)
			},
			run:            func(p *connection.ConnectionPrompterStruct) (interface{}, error) { return p.PromptForRegion("") },
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "region cannot be empty",
		},
		{
			name: "PromptForBastionInstance SSM success",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select bastion instance for SSM:", gomock.Any()).Return("bastion1 (i-123) - No Public IP (SSM only)", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				instances := []models.EC2Instance{{Name: "bastion1", InstanceID: "i-123", PublicIPAddress: ""}} // No public IP
				return p.PromptForBastionInstance(instances, true)
			},
			wantResult:     "i-123",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForBastionInstance SSM all have public IPs",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				instances := []models.EC2Instance{{Name: "bastion1", InstanceID: "i-123", PublicIPAddress: "1.2.3.4"}}
				return p.PromptForBastionInstance(instances, true)
			},
			wantResult:     "",
			wantErr:        true,
			wantErrMessage: "no instances available for SSM connection (all instances have public IPs)",
		},

		{
			name: "PromptForBastionInstance success InstanceID",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("Instance ID (EC2 Instance Connect)", nil)
				m.EXPECT().PromptForSelection("Select bastion instance:", gomock.Any()).Return("bastion1 (i-123) - No Public IP (EC2 Connect only)", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				instances := []models.EC2Instance{{Name: "bastion1", InstanceID: "i-123", PublicIPAddress: ""}}
				return p.PromptForBastionInstance(instances, false)
			},
			wantResult:     "i-123",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForBastionInstance success InstanceID with public IP",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("Instance ID (EC2 Instance Connect)", nil)
				m.EXPECT().PromptForSelection("Select bastion instance:", gomock.Any()).Return("bastion1 (i-123) - 1.2.3.4", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				instances := []models.EC2Instance{{Name: "bastion1", InstanceID: "i-123", PublicIPAddress: "1.2.3.4"}}
				return p.PromptForBastionInstance(instances, false)
			},
			wantResult:     "i-123",
			wantErr:        false,
			wantErrMessage: "",
		},
		{
			name: "PromptForBastionInstance success InstanceID with unnamed instance",
			setup: func(t *testing.T, p *connection.ConnectionPrompterStruct, m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select connection method:", gomock.Any()).Return("Instance ID (EC2 Instance Connect)", nil)
				m.EXPECT().PromptForSelection("Select bastion instance:", gomock.Any()).Return("i-123 (i-123) - 1.2.3.4", nil)
			},
			run: func(p *connection.ConnectionPrompterStruct) (interface{}, error) {
				instances := []models.EC2Instance{{Name: "", InstanceID: "i-123", PublicIPAddress: "1.2.3.4"}}
				return p.PromptForBastionInstance(instances, false)
			},
			wantResult:     "i-123",
			wantErr:        false,
			wantErrMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
			prompter := &connection.ConnectionPrompterStruct{Prompter: mockPrompter}

			if tt.setup != nil {
				tt.setup(t, prompter, mockPrompter)
			}

			result, err := tt.run(prompter)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMessage != "" {
					if errors.Is(err, promptUtils.ErrInterrupted) {
						assert.ErrorIs(t, err, promptUtils.ErrInterrupted)
					} else {
						assert.Contains(t, err.Error(), tt.wantErrMessage)
					}
				}
			} else {
				assert.NoError(t, err)
			}

			switch expected := tt.wantResult.(type) {
			case string:
				assert.Equal(t, expected, result)
			case int:
				if tt.name == "PromptForLocalPort success" {
					assert.True(t, result.(int) >= expected && result.(int) <= 65535)
				} else {
					assert.Equal(t, expected, result)
				}
			case bool:
				assert.Equal(t, expected, result)
			case *connection.ConnectionPrompterStruct:
				assert.NotNil(t, result)
				assert.IsType(t, expected, result)
			default:
				t.Fatalf("unsupported result type: %T", tt.wantResult)
			}
		})
	}
}
