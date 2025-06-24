package models

type Identity struct {
	UserID string `json:"UserId"`
}

type Account struct {
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
}

type AccountNameResponse struct {
	AccountList []Account `json:"accountList"`
}
