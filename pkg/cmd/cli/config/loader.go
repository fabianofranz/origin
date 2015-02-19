package config

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
)

func NewOpenShiftClientConfigLoadingRules() *clientcmd.ClientConfigLoadingRules {
	loadingRules := clientcmd.NewClientConfigLoadingRules()

	envVarPath := os.Getenv(OpenShiftConfigPathEnvVar)
	currentDirectoryPath := OpenShiftConfigFileName
	homeDirectoryPath := fmt.Sprintf("%v/%v/%v", os.Getenv("HOME"), OpenShiftConfigHomeDir, OpenShiftConfigFileName)

	loadingRules.Add("", envVarPath, currentDirectoryPath, homeDirectoryPath)
	return loadingRules
}
