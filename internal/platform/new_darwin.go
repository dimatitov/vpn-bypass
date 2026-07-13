//go:build darwin

package platform

func NewRouter() (Router, error) {
	return darwinRouter{}, nil
}
