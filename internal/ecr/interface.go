package ecr

import (
	"context"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

type ECRAPI interface {
	GetAuthorizationToken(ctx context.Context, params *ecr.GetAuthorizationTokenInput, optFns ...func(*ecr.Options)) (*ecr.GetAuthorizationTokenOutput, error)
}

type ECRAdapterInterface interface {
	Login(ctx context.Context) error
}

type ConfigLoader interface {
	LoadDefaultConfig(ctx context.Context, opts ...func(*config.LoadOptions) error) (aws.Config, error)
}

type ECRServiceInterface interface {
	Run() error
}

type ECRClientFactory interface {
	NewECRClient(cfg aws.Config, fs common.FileSystemInterface, executor sso.CommandExecutor) ECRAdapterInterface
}

type ProfileProvider interface {
	ValidProfiles() ([]string, error)
}

type ECRPromptInterface interface {
	SelectECRAction() (ECRAction, error)
	PromptForProfile() (string, error)
}
