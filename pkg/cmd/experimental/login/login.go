package login

import (
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

type loginOptions struct {
	cmd *cobra.Command

	username string
	password string
	server   string
	project  string
	context  string

	usernameProvided bool
	passwordProvided bool
	serverProvided   bool
	projectProvided  bool
	contextProvided  bool

	configStore *config.ConfigFromFile
	oClient     *client.Client
}

func NewCmdLogin(f *osclientcmd.Factory, parentName, name string) *cobra.Command {
	options := &loginOptions{}

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

			oClient, _, err := f.Clients(cmd)
			if err != nil {
				glog.Fatalf("%v\n", err)
			}

			err = options.fill(cmd, oClient)
			if err != nil {
				glog.Fatalf("%v\n", err)
			}

			glog.V(4).Infof("Using config from %v\n", options.configStore.Path)

			// check to see if we're already signed in. If --username, make sure we are signed in with it. If so, simply make sure that .kubeconfig has that information
			if userFullName, err := whoami(oClient); err == nil && (!options.usernameProvided || (options.usernameProvided && options.username == userFullName)) {
				if err := config.UpdateConfigFile(userFullName, clientCfg.BearerToken, f.OpenShiftClientConfig, options.configStore); err != nil {
					glog.Fatalf("%v\n", err)
				}
				fmt.Printf("Already logged into %v as '%v'\n", clientCfg.Host, userFullName)

			} else {
				requiresNewLogin := true

				if options.usernameProvided {
					glog.V(5).Infof("Checking existing authentication info for '%v'...\n", options.username)

					for key, authInfo := range options.configStore.Config.AuthInfos {
						glog.V(5).Infof("Checking %v...\n", key)

						if key == options.username && len(authInfo.Token) > 0 { // only token for now
							glog.V(5).Infof("Authentication exists for '%v', trying to use it...\n", options.username)

							clientCfg.BearerToken = authInfo.Token
							if userFullName, err := whoami(oClient); err == nil && userFullName == options.username {
								err = config.UpdateConfigFile(userFullName, authInfo.Token, f.OpenShiftClientConfig, options.configStore)
								if err != nil {
									glog.Fatalf("%v\n", err)
								} else {
									requiresNewLogin = false
									fmt.Printf("Already logged into %v as '%v'\n", clientCfg.Host, userFullName)
								}
							} else {
								glog.V(5).Infof("Token %v no longer valid for '%v', need to log in again\n", authInfo.Token, options.username)
							}
						}
					}
				}

				if requiresNewLogin {
					clientCfg.BearerToken = ""
					accessToken, err := tokencmd.RequestToken(clientCfg, os.Stdin, options.username, options.password)

					if err != nil {
						glog.Fatalf("%v\n", err)
					}

					if userFullName, err := whoami(oClient); err == nil {
						err = config.UpdateConfigFile(userFullName, accessToken, f.OpenShiftClientConfig, options.configStore)
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
			// TODO must properly handle policies (projects an user belongs to) https://github.com/openshift/origin/pull/971
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

	cmds.Flags().StringVarP(&options.username, "username", "u", "", "Username, will prompt if not provided")
	cmds.Flags().StringVarP(&options.password, "password", "p", "", "Password, will prompt if not provided")
	cmds.Flags().StringVar(&options.project, "project", "", "If provided the client will switch to use the provided project after logging in")
	// TODO should explicitly expose the flags below as local (currently global)
	cmds.Flags().StringVar(&options.context, "context", "", "Use a specific config context to log in")
	cmds.Flags().StringVar(&options.server, "server", "", "The address and port of the API server to log in to")
	return cmds
}

func whoami(oClient *client.Client) (string, error) {
	me, err := oClient.Users().Get("~")
	if err != nil {
		return "", err
	}

	return me.FullName, nil
}

func (o *loginOptions) fill(cmd *cobra.Command, oClient *client.Client) error {
	o.cmd = cmd

	o.usernameProvided = len(o.username) > 0
	o.passwordProvided = len(o.password) > 0
	o.serverProvided = len(o.server) > 0
	o.projectProvided = len(o.project) > 0
	o.contextProvided = len(o.context) > 0

	configStore, err := config.GetConfigFromDefaultLocations(cmd)
	if err != nil {
		return err
	}
	o.configStore = configStore

	if o.contextProvided {
		ctx, ctxExists := o.configStore.Config.Contexts[o.context]

		// context must exist
		if !ctxExists {
			return fmt.Errorf("Context '%s' not found\n", o.context)
		}

		// if context provides a given information then that flag is mutually exclusive
		if (o.usernameProvided || o.passwordProvided) && len(ctx.AuthInfo) > 0 {
			return fmt.Errorf("The 'context' flag cannot be used with 'username' or 'password' since it already provides these information. You must either provide a context or username and password.")
		}
		if o.serverProvided && len(ctx.Cluster) > 0 {
			return fmt.Errorf("The 'context' flag cannot be used with 'server' since it already provides this information. You must either provide a context or server.")
		}
		if o.projectProvided && len(ctx.Namespace) > 0 {
			return fmt.Errorf("The 'context' flag cannot be used with 'project' since it already provides this information. You must either provide a context or project.")
		}

		// if context provided, take everything from it
		o.username = ctx.AuthInfo
		o.password = ""
		o.server = ctx.Cluster
		o.project = ctx.Namespace
	}

	// project must exist if provided
	if o.projectProvided {
		_, err := oClient.Projects().Get(o.project)
		if err != nil {
			if kerrors.IsNotFound(err) {
				return fmt.Errorf("Project '%s' not found\n", o.project)
			}
			return err
		}
	}

	return nil
}
