package main

import (
	"github.com/bocloud/carina/cmd/carina-controller/run"
	"github.com/bocloud/carina/utils/log"
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
