package cmd

import (
	"fmt"
	"io"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	glog "github.com/openshift/origin/pkg/cmd/util/log"
	"github.com/spf13/cobra"
)

func NewCmdProject(f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <project-name>",
		Short: "switch to another project",
		Long:  `Switch to another project and make it the default in your configuration.`,
		Run: func(cmd *cobra.Command, args []string) {
			argsLength := len(args)

			if argsLength > 1 {
				glog.Fatal("Only one argument is supported (project name).")
			}

			rawCfg, err := f.OpenShiftClientConfig.RawConfig()
			checkErr(err)

			clientCfg, err := f.OpenShiftClientConfig.ClientConfig()
			checkErr(err)

			oClient, _, err := f.Clients(cmd)
			checkErr(err)

			if argsLength == 0 {
				currentContext := rawCfg.Contexts[rawCfg.CurrentContext]
				currentProject := currentContext.Namespace

				_, err := oClient.Projects().Get(currentProject)
				if err != nil {
					if errors.IsNotFound(err) {
						glog.Fatalf("The project '%v' specified in your config does not exist.", currentProject)
					}
					checkErr(err)
				}

				glog.Successf("Using project '%v'.", currentProject)

			} else {
				projectName := args[0]

				project, err := oClient.Projects().Get(projectName)
				if err != nil {
					if errors.IsNotFound(err) {
						glog.Fatalf("Unable to find a project with name '%v'.", projectName)
					}
					checkErr(err)
				}

				configStore, err := config.GetConfigFromDefaultLocations(clientCfg, cmd)
				checkErr(err)

				config := configStore.Config

				// check if context exists in the file I'm going to save
				// if so just set it as the current one
				exists := false
				for k, ctx := range config.Contexts {
					namespace := ctx.Namespace
					cluster := config.Clusters[ctx.Cluster]
					authInfo := config.AuthInfos[ctx.AuthInfo]

					if namespace == project.Name && cluster.Server == clientCfg.Host && authInfo.Token == clientCfg.BearerToken {
						exists = true
						config.CurrentContext = k
						break
					}
				}

				currentCtx := rawCfg.CurrentContext

				// otherwise use the current context if it's in the file I'm going to save,
				// or create a new one if it's not
				if !exists {
					if ctx, ok := config.Contexts[currentCtx]; ok {
						ctx.Namespace = project.Name
						config.Contexts[currentCtx] = ctx
					} else {
						ctx = rawCfg.Contexts[currentCtx]
						newCtx := clientcmdapi.NewContext()
						newCtx.Namespace = project.Name
						newCtx.AuthInfo = ctx.AuthInfo
						newCtx.Cluster = ctx.Cluster
						config.Contexts[fmt.Sprint(util.NewUUID())] = *newCtx
					}
				}

				if err = kclientcmd.WriteToFile(*config, configStore.Path); err != nil {
					glog.Fatalf("Error saving project information in the config: %v.", err)
				}

				glog.Successf("Now using project '%v'.", project.Name)

			}
		},
	}
	return cmd
}
