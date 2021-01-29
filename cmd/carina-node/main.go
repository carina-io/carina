package main

import (
	"carina/cmd/carina-node/run"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	run.Execute()
}
