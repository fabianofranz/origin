package cmd

import (
	"fmt"
	"io"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/spf13/cobra"
)

func NewCmdProject(f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <project-name>",
		Short: "switch to another project",
		Long:  `Switch to another project and make it the default in your configuration.`,
		Run: func(cmd *cobra.Command, args []string) {
			argsLength := len(args)
			switch {
			case argsLength == 0:
				current, err := f.OpenShiftClientConfig.Namespace()
				checkErr(err)
				fmt.Printf("Using project '%v'.\n", current)
			case argsLength > 1:
				glog.Fatalf("Only one argument is supported (project name).")
			default:
				clientCfg, err := f.OpenShiftClientConfig.ClientConfig()
				if err != nil {
					glog.Fatalf("%v", err)
				}

				projectName := args[0]

				oClient, _, err := f.Clients(cmd)
				if err != nil {
					glog.Fatalf("Error getting client: %v", err)
				}

				project, err := oClient.Projects().Get(projectName)
				if err != nil {
					if errors.IsNotFound(err) {
						glog.Fatalf("Unable to find a project with name '%v'.", projectName)
					}
					glog.Fatalf("%v", err)
				}

				configStore, err := config.GetConfigFromDefaultLocations(clientCfg, cmd)
				if err != nil {
					glog.Fatalf("%v", err)
				}

				cfg := configStore.Config
				currentCtx := cfg.Contexts[cfg.CurrentContext]
				currentCtx.Namespace = project.Name
				cfg.Contexts[cfg.CurrentContext] = currentCtx

				if err = kclientcmd.WriteToFile(*cfg, configStore.Path); err != nil {
					glog.Fatalf("Error saving project information in the config: %v", err)
				}

				fmt.Printf("Now using project '%v'.\n", project.Name)
			}
		},
	}

	return cmd
}
