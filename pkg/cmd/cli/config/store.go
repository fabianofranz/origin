package config

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
)

const (
	OpenShiftConfigPathEnvVar      = "OPENSHIFTCONFIG"
	OpenShiftConfigFlagName        = "config"
	OpenShiftConfigFileName        = ".openshiftconfig"
	OpenShiftConfigHomeDir         = ".config/openshift"
	OpenShiftConfigHomeFileName    = ".config"
	OpenShiftConfigHomeDirFileName = OpenShiftConfigHomeDir + "/" + OpenShiftConfigHomeFileName

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

type ConfigStoreNotFoundError struct {
	msg string
}

func (e *ConfigStoreNotFoundError) Error() string {
	return e.msg
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

	// try ~/.config/openshift/.config, if not move on
	path = fmt.Sprintf("%v/%v", os.Getenv("HOME"), OpenShiftConfigHomeDirFileName)
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

	err = &ConfigStoreNotFoundError{"Config file not found in any of the expected locations."}
	glog.V(4).Infof(err.Error())

	return nil, err
}

func tryToLoad(path string, providerEngine string, providerLocation string) (*ConfigStore, error) {
	if len(path) > 0 {
		config, err := clientcmd.LoadFromFile(path)
		if err == nil {
			glog.V(4).Infof("Using config from %v", path)
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

func UpdateConfigFile(username, token string, clientCfg clientcmd.ClientConfig, configStore *ConfigStore) error {
	glog.V(4).Infof("Trying to merge and update %v:%v config to '%v'...", configStore.providerEngine, configStore.providerLocation, configStore.Path)

	rawMergedConfig, err := clientCfg.RawConfig()
	if err != nil {
		return err
	}
	clientConfig, err := clientCfg.ClientConfig()
	if err != nil {
		return err
	}
	namespace, err := clientCfg.Namespace()
	if err != nil {
		return err
	}

	config := clientcmdapi.NewConfig()

	credentialsName := username
	if len(credentialsName) == 0 {
		credentialsName = "osc-login"
	}
	credentials := clientcmdapi.NewAuthInfo()
	credentials.Token = token
	config.AuthInfos[credentialsName] = *credentials

	serverAddr := flagtypes.Addr{Value: clientConfig.Host}.Default()
	clusterName := fmt.Sprintf("%v:%v", serverAddr.Host, serverAddr.Port)
	cluster := clientcmdapi.NewCluster()
	cluster.Server = clientConfig.Host
	cluster.InsecureSkipTLSVerify = clientConfig.Insecure
	cluster.CertificateAuthority = clientConfig.CAFile
	config.Clusters[clusterName] = *cluster

	contextName := clusterName + "-" + credentialsName
	context := clientcmdapi.NewContext()
	context.Cluster = clusterName
	context.AuthInfo = credentialsName
	context.Namespace = namespace
	config.Contexts[contextName] = *context

	config.CurrentContext = contextName

	configToModify := configStore.Config

	configToWrite, err := MergeConfig(rawMergedConfig, *configToModify, *config)
	if err != nil {
		return err
	}

	// TODO need to handle file not writable (probably create a copy)
	err = clientcmd.WriteToFile(*configToWrite, configStore.Path)
	if err != nil {
		return err
	}

	return nil
}

func CreateNewEmptyConfig() (*ConfigStore, error) {
	configPathToCreateIfNotFound := fmt.Sprintf("%v/%v", os.Getenv("HOME"), OpenShiftConfigHomeDirFileName)

	glog.V(3).Infof("A new config will be created at: %v ", configPathToCreateIfNotFound)

	newConfig := clientcmdapi.NewConfig()

	if err := os.MkdirAll(fmt.Sprintf("%v/%v", os.Getenv("HOME"), OpenShiftConfigHomeDir), 0755); err != nil {
		return nil, fmt.Errorf("Tried to create a new config file but failed while creating directory %v: %v", OpenShiftConfigHomeDirFileName, err)
	}
	glog.V(5).Infof("Created directory %v", "~/"+OpenShiftConfigHomeDir)

	if err := clientcmd.WriteToFile(*newConfig, configPathToCreateIfNotFound); err != nil {
		return nil, fmt.Errorf("Tried to create a new config file but failed with: %v", err)
	}
	glog.V(5).Infof("Created file %v", configPathToCreateIfNotFound)

	config, err := tryToLoad(configPathToCreateIfNotFound, fromOpenShift, fromHomeDir)
	if err != nil {
		return nil, err
	}

	return config, nil
}
