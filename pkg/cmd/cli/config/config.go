package config

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
)

func UpdateConfigFile(username, token string, clientCfg clientcmd.ClientConfig, configStore *ConfigStore) error {
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
