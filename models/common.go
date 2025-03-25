package models

// struct for caller identity valid
type Identity struct {
	UserID string `json:"UserId"`
}

// struct for account name response
type AccountNameResponse struct {
	AccountList []struct {
		AccountID   string `json:"accountId"`
		AccountName string `json:"accountName"`
	} `json:"accountList"`
}
