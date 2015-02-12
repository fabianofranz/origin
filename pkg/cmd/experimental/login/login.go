package login

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
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

			configFile, err := config.GetConfigFromDefaultLocations(cmd)
			if err != nil {
				glog.Fatalf("%v\n", err)
			}
			glog.V(4).Infof("Using config from %v\n", configFile.Path)

			// check to see if we're already signed in.  If so, simply make sure that .kubeconfig has that information
			if userFullName, err := whoami(clientCfg); err == nil && (!usernameFlagProvided || (usernameFlagProvided && usernameFlag == userFullName)) {
				if err := config.UpdateConfigFile(userFullName, clientCfg.BearerToken, f.OpenShiftClientConfig, configFile); err != nil {
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
					err = config.UpdateConfigFile(userFullName, accessToken, f.OpenShiftClientConfig, configFile)
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
	// TODO should explicitly expose --server (currently global)
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
