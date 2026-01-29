//go:build !linux

package cli

func setupNetworkManager() error {
	reloadNetworkManager()
	return nil
}

func RestoreNetworkManager() error {
	reloadNetworkManager()
	return nil
}

func reloadNetworkManager() {}
