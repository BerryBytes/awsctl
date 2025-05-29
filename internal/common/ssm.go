package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type RealSSMStarter struct {
	Client          SSMClientInterface
	Region          string
	CommandExecutor common.CommandExecutor
}

func NewRealSSMStarter(client SSMClientInterface, region string) *RealSSMStarter {
	return &RealSSMStarter{
		Client:          client,
		Region:          region,
		CommandExecutor: &common.RealCommandExecutor{},
	}
}

func (s *RealSSMStarter) StartSession(ctx context.Context, instanceID string) error {
	session, err := s.Client.StartSession(ctx, &ssm.StartSessionInput{
		Target: aws.String(instanceID),
	})
	if err != nil {
		return fmt.Errorf("SSM session failed: %w", err)
	}
	defer s.TerminateSession(ctx, session.SessionId)

	fmt.Printf("Starting SSM session with instance %s...\n", instanceID)
	return s.RunSessionManagerPlugin(ctx, session, instanceID, "StartSession")
}

func (s *RealSSMStarter) StartPortForwarding(ctx context.Context, instanceID string, localPort int, remoteHost string, remotePort int) error {
	fmt.Printf("remote host %s", remoteHost)
	session, err := s.Client.StartSession(ctx, &ssm.StartSessionInput{
		Target:       aws.String(instanceID),
		DocumentName: aws.String("AWS-StartPortForwardingSessionToRemoteHost"),
		Parameters: map[string][]string{
			"portNumber":      {fmt.Sprintf("%d", remotePort)},
			"localPortNumber": {fmt.Sprintf("%d", localPort)},
			"host":            {remoteHost},
		},
	})
	if err != nil {
		return fmt.Errorf("SSM port forwarding failed: %w", err)
	}
	defer s.TerminateSession(ctx, session.SessionId)

	fmt.Printf("Starting SSM port forwarding for instance %s (localhost:%d to %s:%d)...\n", instanceID, localPort, remoteHost, remotePort)
	return s.RunSessionManagerPlugin(ctx, session, instanceID, "PortForwarding")
}

func (s *RealSSMStarter) StartSOCKSProxy(ctx context.Context, instanceID string, localPort int) error {
	session, err := s.Client.StartSession(ctx, &ssm.StartSessionInput{
		Target:       aws.String(instanceID),
		DocumentName: aws.String("AWS-StartPortForwardingSession"),
		Parameters: map[string][]string{
			"portNumber":      {"1080"},
			"localPortNumber": {fmt.Sprintf("%d", localPort)},
		},
	})
	if err != nil {
		return fmt.Errorf("SSM SOCKS proxy failed: %w", err)
	}
	defer s.TerminateSession(ctx, session.SessionId)

	fmt.Printf("Starting SSM SOCKS proxy for instance %s on localhost:%d...\n", instanceID, localPort)
	return s.RunSessionManagerPlugin(ctx, session, instanceID, "SOCKSProxy")
}

func (s *RealSSMStarter) TerminateSession(ctx context.Context, sessionID *string) {
	if sessionID == nil {
		return
	}
	_, err := s.Client.TerminateSession(ctx, &ssm.TerminateSessionInput{
		SessionId: sessionID,
	})
	if err != nil {
		fmt.Printf("Warning: Failed to terminate SSM session %s: %v\n", *sessionID, err)
	}
}

func (s *RealSSMStarter) RunSessionManagerPlugin(ctx context.Context, session *ssm.StartSessionOutput, instanceID, sessionType string) error {
	pluginName := "session-manager-plugin"
	if runtime.GOOS == "windows" {
		pluginName = "session-manager-plugin.exe"
	}

	if customPath := os.Getenv("AWS_SESSION_MANAGER_PLUGIN_PATH"); customPath != "" {
		pluginName = customPath
	}

	pluginPath, err := s.CommandExecutor.LookPath(pluginName)
	if err != nil {
		return fmt.Errorf("session-manager-plugin not found. Install it from below: \n%s(%w)",
			"https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html \n", err)
	}

	sessionParams := map[string]interface{}{
		"Target":     instanceID,
		"SessionId":  *session.SessionId,
		"StreamUrl":  *session.StreamUrl,
		"TokenValue": *session.TokenValue,
	}

	paramsJSON, err := json.Marshal(sessionParams)
	if err != nil {
		return fmt.Errorf("failed to marshal session parameters: %w", err)
	}

	args := []string{
		string(paramsJSON),
		s.Region,
		"StartSession",
		"",
		fmt.Sprintf(`{"Target": "%s"}`, instanceID),
	}

	fmt.Printf("Executing %s session for instance %s...\n", sessionType, instanceID)
	return s.CommandExecutor.RunInteractiveCommand(ctx, pluginPath, args...)
}
