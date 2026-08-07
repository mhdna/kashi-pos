package main

import "fmt"

// These variables are intentionally set at build time with -ldflags. The
// default binary is a branch build; central builds can set both values without
// putting the central private key in source control.
var (
	kashiBuildProfile      = "branch"
	kashiCentralPrivateKey = ""
)

func isCentralBuild() bool {
	return kashiBuildProfile == "central"
}

func centralPrivateKey() string {
	if !isCentralBuild() {
		return ""
	}
	return kashiCentralPrivateKey
}

func requireCentralBuild() error {
	if !isCentralBuild() {
		return fmt.Errorf("this operation is available only in the central build")
	}
	return nil
}
