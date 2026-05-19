package httpx

import "context"

type virtualModelContextKey struct{}

func WithVirtualModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, virtualModelContextKey{}, model)
}

func VirtualModelFromContext(ctx context.Context) string {
	model, _ := ctx.Value(virtualModelContextKey{}).(string)
	return model
}
