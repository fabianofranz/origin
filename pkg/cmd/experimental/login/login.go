package login

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

func NewCmdLogin(f *osclientcmd.Factory, parentName, name string) *cobra.Command {
	cmds := &cobra.Command{
		Use:   name,
		Short: "Logs in and returns a session token",
		Long: `Logs in to the OpenShift server and prints out a session token.

Username and password can be provided through flags, the command will
prompt for user input if not provided.
`,
		Run: func(cmd *cobra.Command, args []string) {
			clientCfg, err := f.OpenShiftClientConfig.ClientConfig()
			if err != nil {
				glog.Fatalf("%v\n", err)
			}

			usernameFlag := kubecmd.GetFlagString(cmd, "username")
			passwordFlag := kubecmd.GetFlagString(cmd, "password")

			usernameFlagProvided := len(usernameFlag) > 0

			configFile, err := getConfigFromDefaultLocations(cmd)
			if err != nil {
				glog.Fatalf("%v\n", err)
			}
			glog.V(4).Infof("Using config from %v\n", configFile.path)

			// check to see if we're already signed in.  If so, simply make sure that .kubeconfig has that information
			if userFullName, err := whoami(clientCfg); err == nil && (!usernameFlagProvided || (usernameFlagProvided && usernameFlag == userFullName)) {
				if err := updateKubeconfigFile(userFullName, clientCfg.BearerToken, f.OpenShiftClientConfig, configFile); err != nil {
					glog.Fatalf("%v\n", err)
				}
				fmt.Printf("Already logged into %v as %v\n", clientCfg.Host, userFullName)

			} else {
				clientCfg.BearerToken = ""
				accessToken, err := tokencmd.RequestToken(clientCfg, os.Stdin, usernameFlag, passwordFlag)

				if err != nil {
					glog.Fatalf("%v\n", err)
				}

				if userFullName, err := whoami(clientCfg); err == nil {
					err = updateKubeconfigFile(userFullName, accessToken, f.OpenShiftClientConfig, configFile)
					if err != nil {
						glog.Fatalf("%v\n", err)
					} else {
						fmt.Printf("Logged into %v as %v\n", clientCfg.Host, userFullName)
					}
				} else {
					glog.Fatalf("%v\n", err)
				}
			}
		},
	}

	cmds.Flags().StringP("username", "u", "", "Username, will prompt if not provided")
	cmds.Flags().StringP("password", "p", "", "Password, will prompt if not provided")
	return cmds
}

func whoami(clientCfg *kclient.Config) (string, error) {
	osClient, err := client.New(clientCfg)
	if err != nil {
		return "", err
	}

	me, err := osClient.Users().Get("~")
	if err != nil {
		return "", err
	}

	return me.FullName, nil
}

// Copy of kubectl/cmd/DefaultClientConfig, using NewNonInteractiveDeferredLoadingClientConfig
// TODO find and merge duplicates, this is also in other places
func defaultClientConfig(flags *pflag.FlagSet) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewClientConfigLoadingRules()
	loadingRules.EnvVarPath = os.Getenv(clientcmd.RecommendedConfigPathEnvVar)
	flags.StringVar(&loadingRules.CommandLinePath, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")

	overrides := &clientcmd.ConfigOverrides{}
	overrideFlags := clientcmd.RecommendedConfigOverrideFlags("")
	overrideFlags.ContextOverrideFlags.NamespaceShort = "n"
	clientcmd.BindOverrideFlags(overrides, flags, overrideFlags)
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	return clientConfig
}

// TODO logic below should have its own package

func updateKubeconfigFile(username, token string, clientCfg clientcmd.ClientConfig, configFile *configFile) error {
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

	configToModify := configFile.config

	configToWrite, err := MergeConfig(rawMergedConfig, *configToModify, *config)
	if err != nil {
		return err
	}
	err = clientcmd.WriteToFile(*configToWrite, configFile.path)
	if err != nil {
		return err
	}

	return nil

}

type configFile struct {
	config *clientcmdapi.Config
	path   string
}

func getConfigFromDefaultLocations(cmd *cobra.Command) (*configFile, error) {
	// --kubeconfig flag, if provided will only try this one
	if path := kubecmd.GetFlagString(cmd, "kubeconfig"); len(path) > 0 {
		config, err := getConfigFromFile(path)
		if err == nil {
			return &configFile{config, path}, nil
		} else {
			return nil, err
		}
	}

	// KUBECONFIG env var
	path = os.Getenv(clientcmd.RecommendedConfigPathEnvVar)
	if len(path) > 0 {
		config, err = getConfigFromFile(path)
		if err == nil {
			return &configFile{config, path}, nil
		} else {
			glog.V(4).Infof(err.Error())
		}
	}

	// .kubeconfig in the local directory
	path := ".kubeconfig"
	config, err := getConfigFromFile(path)
	if err == nil {
		return &configFile{config, path}, nil
	} else {
		glog.V(4).Infof(err.Error())
	}

	// ~/.kube/.kubeconfig
	path = os.Getenv("HOME") + "/.kube/.kubeconfig"
	config, err = getConfigFromFile(path)
	if err == nil {
		return &configFile{config, path}, nil
	} else {
		glog.V(4).Infof(err.Error())
	}

	return nil, fmt.Errorf("Config file not found in the default locations.")
}

func getConfigFromFile(filename string) (*clientcmdapi.Config, error) {
	var err error
	config, err := clientcmd.LoadFromFile(filename)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func getUniqueName(basename string, existingNames *util.StringSet) string {
	if !existingNames.Has(basename) {
		return basename
	}

	for i := 0; i < 100; i++ {
		trialName := fmt.Sprintf("%v-%d", basename, i)
		if !existingNames.Has(trialName) {
			return trialName
		}
	}

	return string(util.NewUUID())
}
