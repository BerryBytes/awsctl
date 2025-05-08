package sso

import (
	"errors"
	"fmt"
	"slices"

	"github.com/BerryBytes/awsctl/models"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
)

type AWSSelectionClient interface {
	FindProfile(cfg *models.Config, profileName string) (*models.SSOProfile, error)
	FindAccount(profile *models.SSOProfile, accountName string) (*models.SSOAccount, error)
	ExtractAccountNames(profile *models.SSOProfile) []string
	GetUniqueProfiles(cfg *models.Config) ([]string, error)
	SelectProfile(profiles []string) (string, error)
	SelectAccount(accounts []models.SSOAccount) (string, error)
	SelectRole(roles []string) (string, error)
	SelectProfileFromConfig(cfg *models.Config) (*models.SSOProfile, error)
	SelectAccountFromProfile(profile *models.SSOProfile) (*models.SSOAccount, error)
	SelectRoleFromAccount(account *models.SSOAccount) (string, error)
}

type RealAWSSelectionClient struct {
	Prompter promptUtils.Prompter
}

func (c *RealAWSSelectionClient) FindProfile(cfg *models.Config, profileName string) (*models.SSOProfile, error) {
	for _, profile := range cfg.Aws.Profiles {
		if profile.ProfileName == profileName {
			return &profile, nil
		}
	}
	return nil, fmt.Errorf("profile %s not found", profileName)
}

func (c *RealAWSSelectionClient) FindAccount(profile *models.SSOProfile, accountName string) (*models.SSOAccount, error) {
	for _, account := range profile.Accounts {
		if account.AccountName == accountName {
			return &account, nil
		}
	}
	return nil, fmt.Errorf("account %s not found in profile %s", accountName, profile.ProfileName)
}

func (c *RealAWSSelectionClient) ExtractAccountNames(profile *models.SSOProfile) []string {
	accounts := []string{}
	for _, account := range profile.Accounts {
		accounts = append(accounts, account.AccountName)
	}
	return accounts
}

func (c *RealAWSSelectionClient) GetUniqueProfiles(cfg *models.Config) ([]string, error) {
	profiles := []string{}
	for _, profile := range cfg.Aws.Profiles {
		if !slices.Contains(profiles, profile.ProfileName) {
			profiles = append(profiles, profile.ProfileName)
		}
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no profiles found in configuration")
	}
	return profiles, nil
}

func (c *RealAWSSelectionClient) SelectProfile(profiles []string) (string, error) {

	profile, err := c.Prompter.PromptForSelection("Select an AWS SSO Profile", profiles)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {

			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("profile selection aborted: %v", err)
	}
	return profile, nil
}

func (c *RealAWSSelectionClient) SelectAccount(accounts []models.SSOAccount) (string, error) {
	accountNames := make([]string, len(accounts))
	for i, account := range accounts {
		accountNames[i] = account.AccountName
	}

	accountName, err := c.Prompter.PromptForSelection("Select an AWS Account", accountNames)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("account selection aborted: %v", err)
	}
	return accountName, nil
}

func (c *RealAWSSelectionClient) SelectRole(roles []string) (string, error) {
	role, err := c.Prompter.PromptForSelection("Select an AWS Role", roles)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("role selection aborted: %v", err)
	}
	return role, nil
}

func (c *RealAWSSelectionClient) SelectProfileFromConfig(cfg *models.Config) (*models.SSOProfile, error) {
	profiles, err := c.GetUniqueProfiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique profiles: %v", err)
	}
	selectedProfile, err := c.SelectProfile(profiles)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return nil, promptUtils.ErrInterrupted
		}
		return nil, fmt.Errorf("profile selection aborted: %v", err)
	}
	selectedProfileObj, err := c.FindProfile(cfg, selectedProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to find profile: %v", err)
	}
	return selectedProfileObj, nil
}

func (c *RealAWSSelectionClient) SelectAccountFromProfile(profile *models.SSOProfile) (*models.SSOAccount, error) {
	accounts := c.ExtractAccountNames(profile)
	selectedAccount, err := c.Prompter.PromptForSelection("Select AWS Account", accounts)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return nil, promptUtils.ErrInterrupted
		}
		return nil, fmt.Errorf("account selection aborted: %v", err)
	}
	selectedAccountObj, err := c.FindAccount(profile, selectedAccount)
	if err != nil {
		return nil, fmt.Errorf("failed to find account: %v", err)
	}
	return selectedAccountObj, nil
}

func (c *RealAWSSelectionClient) SelectRoleFromAccount(account *models.SSOAccount) (string, error) {
	if len(account.Roles) == 0 {
		fmt.Println("No roles available for this account.")
		return "", nil
	}
	role, err := c.SelectRole(account.Roles)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("role selection aborted: %v", err)
	}
	return role, nil
}
