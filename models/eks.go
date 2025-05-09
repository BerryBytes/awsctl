package models

type EKSCluster struct {
	ClusterName              string
	Endpoint                 string
	Region                   string
	CertificateAuthorityData string
}
