package project

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/spf13/cobra"
)

func NewCmdProject(f *clientcmd.Factory, parentName, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name + " <project-name>",
		Short: "switch to another project",
		Long:  `Switch to another project and set it in the config file`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				glog.Fatalf("You need to provide the project name.")
			}

			projectName := args[0]

			oClient, _, err := f.Clients(cmd)
			if err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}

			// TODO fetch only projects the user has access https://github.com/openshift/origin/pull/971
			project, err := oClient.Projects().Get(projectName)
			if err != nil {
				if errors.IsNotFound(err) {
					glog.Fatalf("Unable to find a project with name '%v'.", projectName)
				} else {
					glog.Fatalf("%v", err)
				}
			}

			clientCfg, err := f.OpenShiftClientConfig.ClientConfig()
			if err != nil {
				glog.Fatalf("%v", err)
			}

			configStore, err := config.GetConfigFromDefaultLocations(clientCfg, cmd)
			if err != nil {
				glog.Fatalf("%v", err)
			}

			currentCtx := configStore.Config.Contexts[configStore.Config.CurrentContext]

			currentCtx.Namespace = project.Name
			configStore.Config.Contexts[configStore.Config.CurrentContext] = currentCtx

			if err = kclientcmd.WriteToFile(*configStore.Config, configStore.Path); err != nil {
				glog.Fatalf("Error saving config to file: %v", err)
			}

			fmt.Printf("Now using project '%v'.\n", project.Name)
		},
	}

	return cmd
}
