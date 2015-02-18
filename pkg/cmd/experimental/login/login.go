package login

import (
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
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

			usernameFlag := kcmdutil.GetFlagString(cmd, "username")
			passwordFlag := kcmdutil.GetFlagString(cmd, "password")
			serverFlag := kcmdutil.GetFlagString(cmd, "server")
			projectFlag := kcmdutil.GetFlagString(cmd, "project")
			contextFlag := kcmdutil.GetFlagString(cmd, "context")

			oClient, _, err := f.Clients(cmd)
			if err != nil {
				glog.Fatalf("%v\n", err)
			}

			validate(oClient, usernameFlag, passwordFlag, serverFlag, projectFlag, contextFlag)

			usernameFlagProvided := len(usernameFlag) > 0

			configFile, err := config.GetConfigFromDefaultLocations(cmd)
			if err != nil {
				glog.Fatalf("%v\n", err)
			}
			glog.V(4).Infof("Using config from %v\n", configFile.Path)

			// check to see if we're already signed in. If --username, make sure we are signed in with it. If so, simply make sure that .kubeconfig has that information
			if userFullName, err := whoami(clientCfg); err == nil && (!usernameFlagProvided || (usernameFlagProvided && usernameFlag == userFullName)) {
				if err := config.UpdateConfigFile(userFullName, clientCfg.BearerToken, f.OpenShiftClientConfig, configFile); err != nil {
					glog.Fatalf("%v\n", err)
				}
				fmt.Printf("Already logged into %v as '%v'\n", clientCfg.Host, userFullName)

			} else {
				requiresNewLogin := true

				if usernameFlagProvided {
					glog.V(5).Infof("Checking existing authentication info for '%v'...\n", usernameFlag)

					for key, authInfo := range configFile.Config.AuthInfos {
						glog.V(5).Infof("Checking %v...\n", key)

						if key == usernameFlag && len(authInfo.Token) > 0 { // only token for now
							glog.V(5).Infof("Authentication exists for '%v', trying to use it...\n", usernameFlag)

							clientCfg.BearerToken = authInfo.Token
							if userFullName, err := whoami(clientCfg); err == nil && userFullName == usernameFlag {
								err = config.UpdateConfigFile(userFullName, authInfo.Token, f.OpenShiftClientConfig, configFile)
								if err != nil {
									glog.Fatalf("%v\n", err)
								} else {
									requiresNewLogin = false
									fmt.Printf("Already logged into %v as '%v'\n", clientCfg.Host, userFullName)
								}
							} else {
								glog.V(5).Infof("Token %v no longer valid for '%v', need to log in again\n", authInfo.Token, usernameFlag)
							}
						}
					}
				}

				if requiresNewLogin {
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
			}

			// select a project to use
			// TODO must properly handle policies (projects an user belongs to)
			projects, err := oClient.Projects().List(labels.Everything(), labels.Everything())
			if err != nil {
				glog.Fatalf("%v\n", err)
			}

			if size := len(projects.Items); size > 0 {
				if size > 1 {
					projectsNames := make([]string, size)
					for i, project := range projects.Items {
						projectsNames[i] = project.Name
					}
					fmt.Printf("Your projects are: %v. You can switch between them using TODO.\n", strings.Join(projectsNames, ", ")) // TODO switch projects
				}
				current, err := f.OpenShiftClientConfig.Namespace()
				if err != nil {
					glog.Fatalf("%v\n", err)
				}
				fmt.Printf("Now using project '%v'\n", current)
			} else {
				// TODO handle no projects
			}
		},
	}

	cmds.Flags().StringP("username", "u", "", "Username, will prompt if not provided")
	cmds.Flags().StringP("password", "p", "", "Password, will prompt if not provided")
	cmds.Flags().StringP("project", "", "", "If provided the client will switch to use the provided project after logging in")
	cmds.Flags().StringP("context", "", "", "Use a config context to log in")
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

func validate(oClient *client.Client, username string, password string, server string, project string, context string) {
	usernameProvided := len(username) > 0
	passwordProvided := len(password) > 0
	serverProvided := len(server) > 0
	projectProvided := len(project) > 0
	contextProvided := len(context) > 0

	// flags are mutually exclusive
	if contextProvided && (usernameProvided || passwordProvided || serverProvided) {
		glog.Fatalf("The 'context' flag cannot be used with 'username', 'password' or 'server'. You must either provide a context or explicit login details.")
	}

	// project must exist if provided
	if projectProvided {
		_, err := oClient.Projects().Get(project)
		if err != nil {
			if kerrors.IsNotFound(err) {
				glog.Fatalf("Project '%s' not found\n", project)
			}
			glog.Fatalf("%v\n", err)
		}
	}
}
