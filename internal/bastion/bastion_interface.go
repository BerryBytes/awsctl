package bastion

import "context"

type BastionServiceInterface interface {
	Run(ctx context.Context) error
}
