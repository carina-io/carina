package main

import (
	"carina/cmd/carina-node/run"
	"carina/utils/log"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var gitCommitID = "dev"

func main() {
	printWelcome()
	run.Execute()
}

func printWelcome() {
	if gitCommitID == "" {
		gitCommitID = "dev"
	}
	log.Info("-------- Welcome to use Carina Node Server --------")
	log.Infof("Git Commit ID : %s", gitCommitID)
	log.Info("------------------------------------")
}
