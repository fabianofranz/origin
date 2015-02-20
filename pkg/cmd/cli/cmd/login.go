package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

type loginOptions struct {
	cmd         *cobra.Command
	configStore *config.ConfigStore
	oClient     *client.Client

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
}

func NewCmdLogin(f *osclientcmd.Factory, out io.Writer) *cobra.Command {
	options := &loginOptions{}

	cmds := &cobra.Command{
		Use:   "login [--username=<username>] [--password=<password>] [--server=<server>]",
		Short: "Logs in and returns a session token",
		Long: `Logs in to the OpenShift server and prints out a session token.

Username and password can be provided through flags, the command will
prompt for user input if not provided.
`,
		Run: func(cmd *cobra.Command, args []string) {
			clientCfg, err := f.OpenShiftClientConfig.ClientConfig()
			if isServerNotFound(err) {
				server := util.PromptForString(os.Stdin, "OpenShift is not configured. Please provide the server URL or hit <enter> to use '%v': ", osclientcmd.DefaultClusterURL)
				if len(server) == 0 {
					server = osclientcmd.DefaultClusterURL
				}
				serverFlag := cmd.Flags().Lookup("server")
				if serverFlag != nil {
					serverFlag.Value.Set(server)
					if err := cmd.Execute(); err != nil {
						os.Exit(1)
					} else {
						os.Exit(0)
					}
				}
			}
			checkErr(err)

			err = options.fill(cmd, clientCfg)
			checkErr(err)

			// check to see if we're already signed in. If --username, make sure we are signed in with it. If so, simply make sure that .kubeconfig has that information
			if userFullName, err := whoami(clientCfg); err == nil && (!options.usernameProvided || (options.usernameProvided && options.username == userFullName)) {
				err = config.UpdateConfigFile(userFullName, clientCfg.BearerToken, f.OpenShiftClientConfig.KClientConfig, options.configStore)
				checkErr(err)
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
							if userFullName, err := whoami(clientCfg); err == nil && userFullName == options.username {
								err = config.UpdateConfigFile(userFullName, authInfo.Token, f.OpenShiftClientConfig.KClientConfig, options.configStore)
								checkErr(err)
								requiresNewLogin = false
								fmt.Printf("Already logged into %v as '%v'\n", clientCfg.Host, userFullName)
							} else {
								glog.V(5).Infof("Token %v no longer valid for '%v', need to log in again\n", authInfo.Token, options.username)
							}
						}
					}
				}

				if requiresNewLogin {
					clientCfg.BearerToken = ""
					accessToken, err := tokencmd.RequestToken(clientCfg, os.Stdin, options.username, options.password)
					checkErr(err)

					if userFullName, err := whoami(clientCfg); err == nil {
						err = config.UpdateConfigFile(userFullName, accessToken, f.OpenShiftClientConfig.KClientConfig, options.configStore)
						checkErr(err)
						fmt.Printf("Logged into %v as %v\n", clientCfg.Host, userFullName)
					} else {
						glog.Fatalf("%v\n", err)
					}
				}
			}

			// select a project to use
			// TODO must properly handle policies (projects an user belongs to) https://github.com/openshift/origin/pull/971
			oClient, err := client.New(clientCfg)
			checkErr(err)
			projects, err := oClient.Projects().List(labels.Everything(), labels.Everything())
			checkErr(err)

			if size := len(projects.Items); size > 0 {
				if size > 1 {
					projectsNames := make([]string, size)
					for i, project := range projects.Items {
						projectsNames[i] = project.Name
					}
					fmt.Printf("Your projects are: %v. You can switch between them using TODO.\n", strings.Join(projectsNames, ", ")) // TODO switch projects
				}
				current, err := f.OpenShiftClientConfig.Namespace()
				checkErr(err)
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
	//cmds.Flags().StringVar(&options.context, "context", "", "Use a specific config context to log in")
	//cmds.Flags().StringVar(&options.server, "server", "", "The address and port of the API server to log in to")
	return cmds
}

func whoami(clientCfg *kclient.Config) (string, error) {
	oClient, err := client.New(clientCfg)
	if err != nil {
		return "", err
	}

	me, err := oClient.Users().Get("~")
	if err != nil {
		return "", err
	}

	return me.FullName, nil
}

func (o *loginOptions) fill(cmd *cobra.Command, clientCfg *kclient.Config) error {
	o.cmd = cmd

	o.usernameProvided = len(o.username) > 0
	o.passwordProvided = len(o.password) > 0
	o.serverProvided = len(o.server) > 0
	o.projectProvided = len(o.project) > 0
	o.contextProvided = len(o.context) > 0

	configStore, err := config.GetConfigFromDefaultLocations(clientCfg, cmd)
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

		// if context were provided, take everything from it
		o.username = ctx.AuthInfo
		o.password = ""
		o.server = ctx.Cluster
		o.project = ctx.Namespace
	}

	// project must exist if provided
	if o.projectProvided {
		oClient, err := client.New(clientCfg)
		if err != nil {
			return err
		}
		_, err = oClient.Projects().Get(o.project)
		if err != nil {
			if kerrors.IsNotFound(err) {
				return fmt.Errorf("Project '%s' not found\n", o.project)
			} else {
				return err
			}
		}
	}

	return nil
}

// TODO yikes refactor (probably use custom type for error)
func isServerNotFound(e error) bool {
	return e != nil && strings.Contains(e.Error(), "OpenShift is not configured")
}
