package main

import (
	"bocloud.com/cloudnative/carina/cmd/carina-node/run"
	"bocloud.com/cloudnative/carina/utils/log"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"os"
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
	log.Infof("node name : %s", os.Getenv("NODE_NAME"))
	log.Info("------------------------------------")
}
