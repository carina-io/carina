package main

import (
	"bocloud.com/cloudnative/carina/cmd/carina-controller/run"
	"bocloud.com/cloudnative/carina/utils/log"
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
	log.Info("-------- Welcome to use Carina Controller Server --------")
	log.Infof("Git Commit ID : %s", gitCommitID)
	log.Info("------------------------------------")
}
