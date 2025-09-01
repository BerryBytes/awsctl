package sso

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func ValidateAccountID(accountID string) error {
	if len(accountID) != 12 || !regexp.MustCompile(`^\d{12}$`).MatchString(accountID) {
		return fmt.Errorf("invalid account ID: %s (must be 12 digits)", accountID)
	}
	return nil
}

func ValidateStartURL(startURL string) error {
	if !strings.HasPrefix(startURL, "https://") {
		return fmt.Errorf("invalid start URL: %s (must start with https://)", startURL)
	}
	return nil
}

func PrintSummary(profileName, sessionName, ssoStartURL, ssoRegion, accountID, roleName, accountName, roleARN, expiration string) {
	fmt.Println("\nAWS SSO Configuration Summary:")
	fmt.Printf("Profile Name:  %s\n", profileName)
	fmt.Printf("SSO Session:   %s\n", sessionName)
	fmt.Printf("SSO Start URL: %s\n", ssoStartURL)
	fmt.Printf("SSO Region:    %s\n", ssoRegion)
	fmt.Printf("Account ID:    %s\n", accountID)
	if accountName != "" {
		fmt.Printf("Account Name:  %s\n", accountName)
	}
	fmt.Printf("Role Name:     %s\n", roleName)
	if roleARN != "" {
		fmt.Printf("Role ARN:      %s\n", roleARN)
	}
	if expiration != "" {
		fmt.Printf("Expires:       %s\n", expiration)
	}
}

func (c *RealSSOClient) listSSOAccounts(region, startURL string) ([]string, error) {
	accessToken, err := c.GetAccessToken(startURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	output, err := c.Executor.RunCommand(
		"aws", "sso", "list-accounts",
		"--region", region,
		"--access-token", accessToken,
		"--output", "json",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %v: %s", err, string(output))
	}

	type account struct {
		AccountId   string `json:"accountId"`
		AccountName string `json:"accountName"`
	}
	type accountList struct {
		AccountList []account `json:"accountList"`
	}
	var result accountList
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse accounts JSON: %v", err)
	}

	if len(result.AccountList) == 0 {
		return nil, fmt.Errorf("no accounts found")
	}

	accounts := make([]string, 0, len(result.AccountList))
	for _, acc := range result.AccountList {
		if acc.AccountId == "" {
			continue
		}
		name := acc.AccountName
		if name == "" {
			name = "Unnamed"
		}
		accounts = append(accounts, fmt.Sprintf("%s (%s)", acc.AccountId, name))
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no valid accounts found")
	}
	return accounts, nil
}

func (c *RealSSOClient) listSSORoles(region, startURL, accountID string) ([]string, error) {
	accessToken, err := c.GetAccessToken(startURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	output, err := c.Executor.RunCommand("aws", "sso", "list-account-roles", "--region", region, "--account-id", accountID, "--access-token", accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %v: %s", err, string(output))
	}

	type role struct {
		RoleName string `json:"roleName"`
	}
	type roleList struct {
		RoleList []role `json:"roleList"`
	}
	var result roleList
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse roles JSON: %v", err)
	}

	if len(result.RoleList) == 0 {
		return nil, fmt.Errorf("no roles found for account %s", accountID)
	}

	roles := make([]string, 0, len(result.RoleList))
	for _, role := range result.RoleList {
		if role.RoleName == "" {
			continue
		}
		roles = append(roles, role.RoleName)
	}

	if len(roles) == 0 {
		return nil, fmt.Errorf("no valid roles found for account %s", accountID)
	}
	return roles, nil
}

func (c *RealSSOClient) selectAccount(region, startURL string) (string, string, error) {
	fmt.Println("\nFetching available AWS accounts...")
	accounts, err := c.listSSOAccounts(region, startURL)
	if err != nil {
		return "", "", fmt.Errorf("error listing accounts: %w", err)
	}
	if len(accounts) == 0 {
		return "", "", fmt.Errorf("no AWS accounts found")
	}

	selectedAccount, err := c.Prompter.SelectFromList("Select an AWS account:", accounts)
	if err != nil {
		return "", "", fmt.Errorf("failed to select account: %w", err)
	}

	parts := strings.SplitN(selectedAccount, " ", 2)
	accountID := parts[0]
	accountName := ""
	if len(parts) > 1 {
		accountName = parts[1]
	}
	accountName = strings.Trim(accountName, "()")

	if err := ValidateAccountID(accountID); err != nil {
		return "", "", err
	}

	return accountID, accountName, nil
}

func (c *RealSSOClient) selectRole(region, startURL, accountID string) (string, error) {
	fmt.Printf("\nFetching available roles for account %s...\n", accountID)
	roles, err := c.listSSORoles(region, startURL, accountID)
	if err != nil {
		return "", fmt.Errorf("error listing roles: %w", err)
	}
	if len(roles) == 0 {
		return "", fmt.Errorf("no roles found for account %s", accountID)
	}

	role, err := c.Prompter.SelectFromList("Select a role:", roles)
	if err != nil {
		return "", fmt.Errorf("failed to select role: %w", err)
	}
	return role, nil
}

func (c *RealSSOClient) generateProfileName(session, account, role string) string {
	sessionSlug := strings.ToLower(strings.ReplaceAll(session, " ", "-"))
	sessionSlug = strings.Trim(sessionSlug, "-")

	accountSlug := strings.ToLower(strings.ReplaceAll(account, " ", "-"))
	accountSlug = strings.Trim(accountSlug, "-")

	accountParts := strings.Split(accountSlug, "-")
	meaningfulParts := []string{}
	for _, part := range accountParts {
		if part != "" && part != "and" && part != "the" && part != "of" {
			meaningfulParts = append(meaningfulParts, part)
			if len(meaningfulParts) >= 3 {
				break
			}
		}
	}

	acctSlug := strings.Join(meaningfulParts, "-")

	roleWords := strings.FieldsFunc(strings.ToLower(role), func(r rune) bool {
		return r == ' ' || r == '-'
	})
	roleParts := []string{}

	roleAbbreviations := map[string]string{
		"administrator":          "adm",
		"administration":         "adm",
		"admin":                  "adm",
		"readonly":               "ro",
		"read-only":              "ro",
		"read":                   "ro",
		"only":                   "",
		"write":                  "wr",
		"full":                   "full",
		"access":                 "acc",
		"management":             "mgmt",
		"manager":                "mgr",
		"developer":              "dev",
		"operations":             "ops",
		"system":                 "sys",
		"network":                "net",
		"database":               "db",
		"security":               "sec",
		"support":                "sup",
		"production":             "prod",
		"professional":           "prof",
		"awsadministratoraccess": "aws-admin-acc",
		"datalake":               "dl",
	}

	for _, word := range roleWords {
		word = strings.ToLower(word)
		if abbrev, exists := roleAbbreviations[word]; exists {
			if abbrev != "" {
				roleParts = append(roleParts, abbrev)
			}
			continue
		}

		if len(word) <= 3 {
			roleParts = append(roleParts, word)
		} else {
			roleParts = append(roleParts, word[:3])
		}
	}

	roleSlug := strings.Join(roleParts, "-")

	profileName := sessionSlug + "-" + acctSlug + "-" + roleSlug

	return profileName
}
