package platform

import "context"

type Gateway struct {
	Address   string
	Interface string
}

type Router interface {
	DirectGateway(ctx context.Context) (Gateway, error)
	RouteFor(ctx context.Context, ip string) (Gateway, error)
	AddRoute(ctx context.Context, prefix string, gateway Gateway) error
	DeleteRoute(ctx context.Context, prefix string, interfaceName string) error
}
