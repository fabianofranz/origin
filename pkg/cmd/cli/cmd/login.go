package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/cli/cmd/setup"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	glog "github.com/openshift/origin/pkg/cmd/util/log"
)

type loginOptions struct {
	username string
	password string
	server   string
	context  string
}

const (
	longDescription = `Logs in to the OpenShift server and prints out a session token.

Username, password and server URL can be provided through flags, the command will
prompt for user input if not provided.
` // TODO better long desc
)

func NewCmdLogin(f *osclientcmd.Factory, in io.Reader, out io.Writer) *cobra.Command {
	options := &loginOptions{}

	cmds := &cobra.Command{
		Use:   "login [--username=<username>] [--password=<password>] [--server=<server>] [--context=<context>]",
		Short: "Logs in and returns a session token",
		Long:  longDescription,
		Run: func(cmd *cobra.Command, args []string) {
			// validate flags
			err := options.validate()
			checkErr(err)

			var clientSetup setup.ClientSetup
			clientSetup = setup.NewOscClientSetup(f, cmd, in, options.username, options.password, options.server, options.context)

			// set up the server
			serverInfo, err := clientSetup.DetermineServerInfo()
			checkErr(err)

			// set up an auth
			authInfo, err := clientSetup.DetermineAuthInfo()
			checkErr(err)
			var loggedMsg string
			if authInfo.NewAuth {
				loggedMsg = "Logged into '%v' as '%v'."
			} else {
				loggedMsg = "Already logged into '%v' as '%v'."
			}
			glog.Infof(loggedMsg, serverInfo.URL, authInfo.FullName)

			// set up projects
			projectInfo, err := clientSetup.DetermineProjectInfo()
			checkErr(err)

			glog.Warningf("Using project '%v'.", projectInfo.ProjectInUse)
			if len(projectInfo.Projects) > 0 {
				glog.Infof("Your projects are: %v. You can switch between them at any time using 'osc project <project-name>'.", strings.Join(projectInfo.Projects, ", ")) // TODO parameterize cmd name
			}

			// merge configs
			err = clientSetup.SaveConfig()
			checkErr(err)

			glog.Success("\nDone! Run 'osc --help' for more information about client commands.")
		},
	}

	// TODO must explicitly expose the flags below as local (currently global)
	cmds.Flags().StringVarP(&options.username, "username", "u", "", "Username, will prompt if not provided")
	cmds.Flags().StringVarP(&options.password, "password", "p", "", "Password, will prompt if not provided")
	//cmds.Flags().StringVar(&options.context, "context", "", "Use a specific config context to log in")
	//cmds.Flags().StringVar(&options.server, "server", "", "The address and port of the API server to log in to")
	return cmds
}

func (o *loginOptions) validate() error {
	if o.contextProvided() {
		// if context provides a given information then that flag is mutually exclusive
		if o.usernameProvided() || o.passwordProvided() {
			return fmt.Errorf("The 'context' flag cannot be used with 'username' or 'password' since it already provides auth information. You must either provide a context or username and password.")
		}
		if o.serverProvided() {
			return fmt.Errorf("The 'context' flag cannot be used with 'server' since it already provides this information. You must either provide a context or server.")
		}
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
