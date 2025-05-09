package eks

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"path/filepath"

	"github.com/BerryBytes/awsctl/models"
	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/smithy-go"
	"gopkg.in/yaml.v3"
)

type EKSClientInterface = EKSAPI

type AwsEKSAdapter struct {
	Client     EKSAPI
	Cfg        aws.Config
	FileSystem common.FileSystemInterface
}

type Kubeconfig struct {
	APIVersion     string         `yaml:"apiVersion"`
	Kind           string         `yaml:"kind"`
	Clusters       []ClusterEntry `yaml:"clusters"`
	Contexts       []ContextEntry `yaml:"contexts"`
	Users          []UserEntry    `yaml:"users"`
	CurrentContext string         `yaml:"current-context"`
}

type ClusterEntry struct {
	Name    string      `yaml:"name"`
	Cluster ClusterData `yaml:"cluster"`
}

type ClusterData struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
}

type UserEntry struct {
	Name string   `yaml:"name"`
	User UserData `yaml:"user"`
}

type UserData struct {
	Exec ExecConfig `yaml:"exec"`
}

type ExecConfig struct {
	APIVersion string   `yaml:"apiVersion"`
	Command    string   `yaml:"command"`
	Args       []string `yaml:"args"`
	Env        []EnvVar `yaml:"env,omitempty"`
}

type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type ContextEntry struct {
	Name    string      `yaml:"name"`
	Context ContextData `yaml:"context"`
}

type ContextData struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

func NewEKSClient(cfg aws.Config, fs common.FileSystemInterface) *AwsEKSAdapter {
	return &AwsEKSAdapter{
		Client:     eks.NewFromConfig(cfg),
		Cfg:        cfg,
		FileSystem: fs,
	}
}

func (c *AwsEKSAdapter) ListClusters(ctx context.Context, input *eks.ListClustersInput, opts ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	return c.Client.ListClusters(ctx, input, opts...)
}

func (c *AwsEKSAdapter) DescribeCluster(ctx context.Context, input *eks.DescribeClusterInput, opts ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	return c.Client.DescribeCluster(ctx, input, opts...)
}

func (c *AwsEKSAdapter) ListEKSClusters(ctx context.Context) ([]models.EKSCluster, error) {
	var clusters []models.EKSCluster

	input := &eks.ListClustersInput{}
	for {
		output, err := c.ListClusters(ctx, input)
		if err != nil {
			return nil, c.HandleAWSError(err, "listing EKS clusters")
		}

		for _, clusterName := range output.Clusters {
			clusterDetails, err := c.GetClusterDetails(ctx, clusterName)
			if err != nil {
				continue
			}
			clusters = append(clusters, *clusterDetails)
		}

		if output.NextToken == nil {
			break
		}
		input.NextToken = output.NextToken
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].ClusterName < clusters[j].ClusterName
	})

	return clusters, nil
}

func (c *AwsEKSAdapter) GetClusterDetails(ctx context.Context, clusterName string) (*models.EKSCluster, error) {
	output, err := c.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return nil, c.HandleAWSError(err, fmt.Sprintf("describing EKS cluster %s", clusterName))
	}

	cluster := output.Cluster
	if cluster == nil || cluster.Endpoint == nil || cluster.CertificateAuthority == nil {
		return nil, fmt.Errorf("invalid cluster data for %s", clusterName)
	}

	return &models.EKSCluster{
		ClusterName:              clusterName,
		Endpoint:                 aws.ToString(cluster.Endpoint),
		Region:                   c.Cfg.Region,
		CertificateAuthorityData: aws.ToString(cluster.CertificateAuthority.Data),
	}, nil
}

func (c *AwsEKSAdapter) UpdateKubeconfig(cluster *models.EKSCluster, profile string) error {
	homeDir, err := c.FileSystem.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")

	var kubeconfig = &Kubeconfig{}
	if _, err := c.FileSystem.Stat(kubeconfigPath); err == nil {
		data, err := c.FileSystem.ReadFile(kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to read kubeconfig: %w", err)
		}
		if err := yaml.Unmarshal(data, kubeconfig); err != nil {
			return fmt.Errorf("failed to parse kubeconfig: %w", err)
		}
	} else {
		kubeconfig.APIVersion = "v1"
		kubeconfig.Kind = "Config"
	}

	clusterEntry := ClusterEntry{
		Name: cluster.ClusterName,
		Cluster: ClusterData{
			Server:                   cluster.Endpoint,
			CertificateAuthorityData: cluster.CertificateAuthorityData,
		},
	}

	userEntry := UserEntry{
		Name: fmt.Sprintf("%s-user", cluster.ClusterName),
		User: UserData{
			Exec: ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1beta1",
				Command:    "aws",
				Args: []string{
					"--region", cluster.Region,
					"eks", "get-token",
					"--cluster-name", cluster.ClusterName,
				},
			},
		},
	}
	if profile != "" {
		userEntry.User.Exec.Env = []EnvVar{
			{Name: "AWS_PROFILE", Value: profile},
		}
	}

	contextEntry := ContextEntry{
		Name: cluster.ClusterName,
		Context: ContextData{
			Cluster: cluster.ClusterName,
			User:    fmt.Sprintf("%s-user", cluster.ClusterName),
		},
	}

	updateOrAppendCluster(&kubeconfig.Clusters, clusterEntry)
	updateOrAppendUser(&kubeconfig.Users, userEntry)
	updateOrAppendContext(&kubeconfig.Contexts, contextEntry)

	kubeconfig.CurrentContext = cluster.ClusterName

	data, err := yaml.Marshal(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}
	if err := c.FileSystem.MkdirAll(filepath.Dir(kubeconfigPath), 0755); err != nil {
		return fmt.Errorf("failed to create kubeconfig directory: %w", err)
	}
	if err := c.FileSystem.WriteFile(kubeconfigPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

func (c *AwsEKSAdapter) HandleAWSError(err error, operation string) error {
	var apiErr *smithy.GenericAPIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "RequestExpired":
			return fmt.Errorf("AWS request expired during %s: %w", operation, err)
		case "AuthFailure", "UnauthorizedOperation":
			return fmt.Errorf("AWS authentication failed during %s: %w", operation, err)
		case "ResourceNotFoundException":
			return fmt.Errorf("EKS resource not found during %s: %w", operation, err)
		}
	}
	return fmt.Errorf("failed during %s: %w", operation, err)
}

func updateOrAppendCluster(clusters *[]ClusterEntry, newEntry ClusterEntry) {
	for i, cluster := range *clusters {
		if cluster.Name == newEntry.Name {
			(*clusters)[i] = newEntry
			return
		}
	}
	*clusters = append(*clusters, newEntry)
}

func updateOrAppendUser(users *[]UserEntry, newEntry UserEntry) {
	for i, user := range *users {
		if user.Name == newEntry.Name {
			(*users)[i] = newEntry
			return
		}
	}
	*users = append(*users, newEntry)
}

func updateOrAppendContext(contexts *[]ContextEntry, newEntry ContextEntry) {
	for i, context := range *contexts {
		if context.Name == newEntry.Name {
			(*contexts)[i] = newEntry
			return
		}
	}
	*contexts = append(*contexts, newEntry)
}
