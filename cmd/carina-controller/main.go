package main

import (
	"carina/cmd/carina-controller/run"
	"carina/utils/log"
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
