package config

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
)

// Set up the rules and priorities for loading config files. The path can be provided through:
// 1. --config flag (this is set in commands)
// 2. OPENSHIFTCONFIG env var
// 3. .openshiftconfig file in current directory
// 4. ~/.config/openshift/.config file
// 5. KUBECONFIG env var
// 6. .kubeconfig file in current directory
// 7. ~/.kube/.kubeconfig file
func NewOpenShiftClientConfigLoadingRules() *clientcmd.ClientConfigLoadingRules {
	// Kube locations (notice we don't expose --kubeconfig)
	loadingRules := clientcmd.NewClientConfigLoadingRules()
	loadingRules.Default().EnvVarPath = os.Getenv(clientcmd.RecommendedConfigPathEnvVar)

	// OpenShift locations
	envVarPath := os.Getenv(OpenShiftConfigPathEnvVar)
	currentDirectoryPath := OpenShiftConfigFileName
	homeDirectoryPath := fmt.Sprintf("%v/%v", os.Getenv("HOME"), OpenShiftConfigHomeDirFileName)
	loadingRules.PrependRule("", envVarPath, currentDirectoryPath, homeDirectoryPath)

	return loadingRules
}
