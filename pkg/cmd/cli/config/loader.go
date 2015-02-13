package config

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
)

func NewOpenShiftClientConfigLoadingRules() *clientcmd.ClientConfigLoadingRules {
	loadingRules := clientcmd.NewClientConfigLoadingRules()

	commandLinePath := "" // we don't have a --openshiftconfig
	envVarPath := OpenShiftConfigPathEnvVar
	currentDirectoryPath := OpenShiftConfigFileName
	homeDirectoryPath := fmt.Sprintf("%v/%v/%v", os.Getenv("HOME"), OpenShiftConfigHomeDir, OpenShiftConfigFileName)

	loadingRules.Add(commandLinePath, envVarPath, currentDirectoryPath, homeDirectoryPath)
	return loadingRules
}
