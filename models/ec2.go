package models

type EC2Instance struct {
	InstanceID       string
	Name             string
	PublicIPAddress  string
	PrivateIPAddress string
	State            string
	InstanceType     string
	AZ               string
	Tags             map[string]string
}
