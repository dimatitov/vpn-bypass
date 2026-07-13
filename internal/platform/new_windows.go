//go:build windows

package platform

func NewRouter() (Router, error) {
	return windowsRouter{}, nil
}
