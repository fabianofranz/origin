package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	userapi "github.com/openshift/origin/pkg/user/api"
)

type loginOptions struct {
	username string
	password string
	server   string
	context  string
}

func NewCmdLogin(f *osclientcmd.Factory, out io.Writer) *cobra.Command {
	options := &loginOptions{}

	cmds := &cobra.Command{
		Use:   "login [--username=<username>] [--password=<password>] [--server=<server>]",
		Short: "Logs in and returns a session token",
		Long: `Logs in to the OpenShift server and prints out a session token.

Username, password and server URL can be provided through flags, the command will
prompt for user input if not provided.
`, // TODO better long desc
		Run: func(cmd *cobra.Command, args []string) {
			// fetch config and if a server were not provided, prompt for it
			clientCfg, err := f.OpenShiftClientConfig.ClientConfig()

			if isServerNotFound(err) {
				defaultServer := osclientcmd.DefaultClusterURL
				promptedServer := util.PromptForStringWithDefault(os.Stdin, defaultServer, "Please provide the OpenShift server URL or hit <enter> to use '%v': ", defaultServer)
				serverFlag := cmd.Flags().Lookup("server")
				if serverFlag != nil {
					serverFlag.Value.Set(promptedServer)
					if err := cmd.Execute(); err != nil {
						os.Exit(1)
					}
					os.Exit(0)
				}
			}

			checkErr(err)

			// try to find a config file from default locations, or create a new one if not found
			configStore, err := config.GetConfigFromDefaultLocations(clientCfg, cmd)
			checkErr(err)

			// validate flags
			checkErr(options.validate(configStore))

			// Check to see if we're already signed in (with the user provided by --username)
			// If so, simply make sure that the config file has that information
			if me, err := whoami(clientCfg); err == nil && (!options.usernameProvided() || (options.usernameProvided() && options.username == usernameFromUser(me))) {
				err = config.UpdateConfigFile(me.FullName, clientCfg.BearerToken, f.OpenShiftClientConfig.KClientConfig, configStore)
				checkErr(err)
				fmt.Printf("Already logged into %v as '%v'\n", clientCfg.Host, me.FullName)

			} else {
				requiresNewLogin := true

				// if trying to log in an user that's not the currently logged in, try to reuse a previous token
				if options.usernameProvided() {
					glog.V(5).Infof("Checking existing authentication info for '%v'...\n", options.username)

					for key, authInfo := range configStore.Config.AuthInfos {
						glog.V(5).Infof("Checking %v...\n", key)

						if key == options.username && len(authInfo.Token) > 0 { // only token for now
							glog.V(5).Infof("Authentication exists for '%v', trying to use it...\n", options.username)

							clientCfg.BearerToken = authInfo.Token
							if me, err := whoami(clientCfg); err == nil && usernameFromUser(me) == options.username {
								err = config.UpdateConfigFile(me.FullName, authInfo.Token, f.OpenShiftClientConfig.KClientConfig, configStore)
								checkErr(err)
								requiresNewLogin = false
								fmt.Printf("Already logged into %v as '%v'\n", clientCfg.Host, me.FullName)
							} else {
								glog.V(5).Infof("Token %v no longer valid for '%v', need to log in again\n", authInfo.Token, options.username)
							}
						}
					}
				}

				// at this point we know we need to log in again
				if requiresNewLogin {
					clientCfg.BearerToken = ""
					accessToken, err := tokencmd.RequestToken(clientCfg, os.Stdin, options.username, options.password)
					if isServerCertificateSignedByUnknownAuthority(err) {
						fmt.Println("The server uses a certificate signed by unknown authority. You can bypass the certificate check but it will make all connections insecure.")
						insecure := util.PromptForBool(os.Stdin, "Use insecure connections [y/N]? ")
						insecureFlag := cmd.Flags().Lookup("insecure-skip-tls-verify")
						if insecure && insecureFlag != nil {
							insecureFlag.Value.Set(strconv.FormatBool(insecure))
							if err := cmd.Execute(); err != nil {
								os.Exit(1)
							}
							os.Exit(0)
						}
					}
					checkErr(err)

					if me, err := whoami(clientCfg); err == nil {
						err = config.UpdateConfigFile(me.FullName, accessToken, f.OpenShiftClientConfig.KClientConfig, configStore)
						checkErr(err)
						fmt.Printf("Logged into %v as %v\n", clientCfg.Host, me.FullName)
					} else {
						glog.Fatalf("%v\n", err)
					}
				}
			}

			// print the user's projects and if there's more than one let them know how to switch
			oClient, err := client.New(clientCfg)
			checkErr(err)
			projects, err := oClient.Projects().List(labels.Everything(), labels.Everything())
			checkErr(err)

			projectsItems := projects.Items
			projectsLength := len(projectsItems)

			switch projectsLength {
			case 0:
				fmt.Printf(`You don't have any project.
Use the 'openshift ex new-project <project-name>' command to create a new project.`) // TODO parameterize cmd name
			case 1:
				fmt.Printf("Using project '%v'\n", projectsItems[0].Name)
			default:
				projectsNames := make([]string, projectsLength)
				for i, project := range projectsItems {
					projectsNames[i] = project.Name
				}
				fmt.Printf("Your projects are: %v. You can switch between them at any time using 'osc project <project-name>'.\n", strings.Join(projectsNames, ", ")) // TODO parameterize cmd name
				current, err := f.OpenShiftClientConfig.Namespace()
				checkErr(err)
				fmt.Printf("Using project '%v'\n", current)
			}

		},
	}

	cmds.Flags().StringVarP(&options.username, "username", "u", "", "Username, will prompt if not provided")
	cmds.Flags().StringVarP(&options.password, "password", "p", "", "Password, will prompt if not provided")
	// TODO should explicitly expose the flags below as local (currently global)
	//cmds.Flags().StringVar(&options.context, "context", "", "Use a specific config context to log in")
	//cmds.Flags().StringVar(&options.server, "server", "", "The address and port of the API server to log in to")
	return cmds
}

func whoami(clientCfg *kclient.Config) (*userapi.User, error) {
	oClient, err := client.New(clientCfg)
	if err != nil {
		return nil, err
	}

	me, err := oClient.Users().Get("~")
	if err != nil {
		return nil, err
	}

	return me, nil
}

func (o *loginOptions) validate(configStore *config.ConfigStore) error {
	if o.contextProvided() {
		ctx, ctxExists := configStore.Config.Contexts[o.context]

		// context must exist
		if !ctxExists {
			return fmt.Errorf("Context '%s' not found\n", o.context)
		}

		// if context provides a given information then that flag is mutually exclusive
		if (o.usernameProvided() || o.passwordProvided()) && len(ctx.AuthInfo) > 0 {
			return fmt.Errorf("The 'context' flag cannot be used with 'username' or 'password' since it already provides these information. You must either provide a context or username and password.")
		}
		if o.serverProvided() && len(ctx.Cluster) > 0 {
			return fmt.Errorf("The 'context' flag cannot be used with 'server' since it already provides this information. You must either provide a context or server.")
		}

		// if context were provided, take everything from it
		o.username = ctx.AuthInfo
		o.password = ""
		o.server = ctx.Cluster
	}

	return nil
}

func (o *loginOptions) usernameProvided() bool {
	return len(o.username) > 0
}

func (o *loginOptions) passwordProvided() bool {
	return len(o.password) > 0
}

func (o *loginOptions) serverProvided() bool {
	return len(o.server) > 0
}

func (o *loginOptions) contextProvided() bool {
	return len(o.context) > 0
}

// TODO yikes refactor (probably use custom type for error)
func isServerNotFound(e error) bool {
	return e != nil && strings.Contains(e.Error(), "OpenShift is not configured")
}

// TODO yikes refactor (probably use custom type for error)
func isServerCertificateSignedByUnknownAuthority(e error) bool {
	return e != nil && strings.Contains(e.Error(), "certificate signed by unknown authority")
}

func usernameFromUser(user *userapi.User) string {
	return strings.Split(user.Name, ":")[1]
}
