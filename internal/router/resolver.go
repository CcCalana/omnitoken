package router

import "context"

type Resolution struct {
	RealModel string
	Provider  string
	IsVirtual bool
}

type Resolver interface {
	Resolve(ctx context.Context, requested string) (Resolution, error)
}
