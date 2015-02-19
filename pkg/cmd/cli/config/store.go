package config

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

const (
	OpenShiftConfigPathEnvVar = "OPENSHIFTCONFIG"
	OpenShiftConfigFlagName   = "config"
	OpenShiftConfigFileName   = ".openshiftconfig"
	OpenShiftConfigHomeDir    = ".openshift"

	KubeConfigPathEnvVar = clientcmd.RecommendedConfigPathEnvVar
	KubeConfigFileName   = ".kubeconfig"
	KubeConfigHomeDir    = ".kube"

	fromFlag     = "flag"
	fromEnvVar   = "envvar"
	fromLocalDir = "localdir"
	fromHomeDir  = "homedir"

	fromKube      = "fromkube"
	fromOpenShift = "fromopenshift"
)

type ConfigStore struct {
	Config           *clientcmdapi.Config
	Path             string
	providerEngine   string
	providerLocation string
}

func (c *ConfigStore) FromFlag() bool {
	return c.providerLocation == fromFlag
}

func (c *ConfigStore) FromEnvVar() bool {
	return c.providerLocation == fromEnvVar
}

func (c *ConfigStore) FromLocalDir() bool {
	return c.providerLocation == fromLocalDir
}

func (c *ConfigStore) FromHomeDir() bool {
	return c.providerLocation == fromHomeDir
}

func (c *ConfigStore) FromOpenShift() bool {
	return c.providerEngine == fromOpenShift
}

func (c *ConfigStore) FromKube() bool {
	return c.providerEngine == fromKube
}

func GetConfigFromDefaultLocations(clientCfg *client.Config, cmd *cobra.Command) (*ConfigStore, error) {
	configPathToCreateIfNotFound := fmt.Sprintf("%v/%v/%v", os.Getenv("HOME"), OpenShiftConfigHomeDir, OpenShiftConfigFileName)

	// --config flag, if provided will only try this one
	path := cmdutil.GetFlagString(cmd, OpenShiftConfigFlagName)
	if len(path) > 0 {
		config, err := tryToLoad(path, fromOpenShift, fromFlag)
		if err == nil {
			return config, nil
		} else {
			return nil, err
		}
	}

	// try openshift env var, if not move on
	path = os.Getenv(OpenShiftConfigPathEnvVar)
	config, err := tryToLoad(path, fromOpenShift, fromEnvVar)
	if err == nil {
		return config, nil
	}

	// try .openshiftconfig in the local directory, if not move on
	path = OpenShiftConfigFileName
	config, err = tryToLoad(path, fromOpenShift, fromLocalDir)
	if err == nil {
		return config, nil
	}

	// try ~/.openshift/.openshiftconfig, if not move on
	path = configPathToCreateIfNotFound
	config, err = tryToLoad(path, fromOpenShift, fromHomeDir)
	if err == nil {
		return config, nil
	}

	// try kube env var, if not move on
	path = os.Getenv(KubeConfigPathEnvVar)
	config, err = tryToLoad(path, fromKube, fromEnvVar)
	if err == nil {
		return config, nil
	}

	// try .kubeconfig in the local directory, if not move on
	path = KubeConfigFileName
	config, err = tryToLoad(path, fromKube, fromLocalDir)
	if err == nil {
		return config, nil
	}

	// try ~/.kube/.kubeconfig, if not move on
	path = fmt.Sprintf("%v/%v/%v", os.Getenv("HOME"), KubeConfigHomeDir, KubeConfigFileName)
	config, err = tryToLoad(path, fromKube, fromHomeDir)
	if err == nil {
		return config, nil
	}

	glog.V(3).Infof("Config file not found in any of the expected locations, a new config will be created: %v ", configPathToCreateIfNotFound)

	newConfig := clientcmdapi.NewConfig()
	if err = clientcmd.WriteToFile(*newConfig, configPathToCreateIfNotFound); err != nil {
		return nil, err
	}

	config, err = tryToLoad(configPathToCreateIfNotFound, fromKube, fromHomeDir)
	if err == nil {
		return config, nil
	} else {
		return nil, fmt.Errorf("Config file not found in any of the default locations. Tried to create but failed with: %v", err)
	}
}

func getConfigFromFile(filename string) (*clientcmdapi.Config, error) {
	var err error
	config, err := clientcmd.LoadFromFile(filename)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func tryToLoad(path string, providerEngine string, providerLocation string) (*ConfigStore, error) {
	if len(path) > 0 {
		config, err := getConfigFromFile(path)
		if err == nil {
			return &ConfigStore{config, path, providerEngine, providerLocation}, nil
		} else {
			glog.V(5).Infof("Unable to load config file for %v:%v: %v", providerEngine, providerLocation, err.Error())
			return nil, fmt.Errorf("Config file not found in %v", path)
		}
	}
	err := fmt.Errorf("Path for %v:%v was empty", providerEngine, providerLocation)
	glog.V(5).Infof(err.Error())
	return nil, err
}
