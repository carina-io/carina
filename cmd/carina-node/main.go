package main

import (
	"carina/cmd/carina-node/cmd"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	cmd.Execute()
}
