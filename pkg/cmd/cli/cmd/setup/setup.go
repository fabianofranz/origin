package setup

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	glog "github.com/openshift/origin/pkg/cmd/util/log"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	"github.com/openshift/origin/pkg/user/api"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
)

const defaultClusterURL = "https://localhost:8443"

type ServerSetupInfo struct {
	URL      string
	Insecure bool
}

type AuthSetupInfo struct {
	Username string
	FullName string
	Token    string
	NewAuth  bool
}

type ProjectSetupInfo struct {
	Projects     []string
	ProjectInUse string
}

// Provide individual and auto-contained steps required to setup a client
type ClientSetup interface {
	DetermineServerInfo() (*ServerSetupInfo, error)
	DetermineAuthInfo() (*AuthSetupInfo, error)
	DetermineProjectInfo() (*ProjectSetupInfo, error)
	MergeConfig() error
}

// Implements a ClientSetup specifically targeted for osc
type OscClientSetup struct {
	reader  io.Reader
	factory *clientcmd.Factory
	cmd     *cobra.Command

	username string
	password string
	server   string
	context  string

	serverSetupInfo  *ServerSetupInfo
	authSetupInfo    *AuthSetupInfo
	projectSetupInfo *ProjectSetupInfo
}

func NewOscClientSetup(f *clientcmd.Factory, cmd *cobra.Command, reader io.Reader, defaultUsername string, defaultPassword string, defaultServer string, defaultContext string) *OscClientSetup {
	return &OscClientSetup{
		reader:           reader,
		factory:          f,
		cmd:              cmd,
		username:         defaultUsername,
		password:         defaultPassword,
		server:           defaultServer,
		context:          defaultContext,
		serverSetupInfo:  &ServerSetupInfo{},
		authSetupInfo:    &AuthSetupInfo{},
		projectSetupInfo: &ProjectSetupInfo{},
	}
}

func (c OscClientSetup) DetermineServerInfo() (*ServerSetupInfo, error) {
	// fetch a client config
	clientCfg, err := c.factory.OpenShiftClientConfig.ClientConfig()

	// if a server were not provided, prompt for it and rerun the command
	if isOpenShiftNotConfigured(err) {
		defaultServer := defaultClusterURL
		promptedServer := util.PromptForStringWithDefault(c.reader, defaultServer, "Please provide the OpenShift server URL or hit <enter> to use '%v': ", defaultServer)
		serverFlag := c.cmd.Flags().Lookup("server")
		if serverFlag != nil {
			serverFlag.Value.Set(promptedServer)
			if err := c.cmd.Execute(); err != nil {
				glog.Fatal(err)
			}
			os.Exit(0)
		}
	}

	if err != nil {
		return nil, err
	}

	c.serverSetupInfo.URL = clientCfg.Host

	return c.serverSetupInfo, nil
}

func (c OscClientSetup) DetermineAuthInfo() (*AuthSetupInfo, error) {
	clientCfg, err := c.factory.OpenShiftClientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	// Check to see if we're already signed in (with the user provided)
	// If so, simply return the existing auth info
	if me, err := whoami(clientCfg); err == nil && (!c.usernameProvided() || (c.usernameProvided() && c.username == usernameFromUser(me))) {
		c.authSetupInfo.Username = usernameFromUser(me)
		c.authSetupInfo.FullName = me.FullName
		c.authSetupInfo.Token = clientCfg.BearerToken
		c.authSetupInfo.NewAuth = false
		return c.authSetupInfo, nil
	}

	configStore, err := config.GetConfigFromDefaultLocations(clientCfg, c.cmd)
	if err != nil {
		if _, isNotFound := err.(*config.ConfigStoreNotFoundError); !isNotFound {
			return nil, err
		}
	}

	// if trying to log in an user that's not the currently logged in, try to reuse an existing token
	if configStore != nil && c.usernameProvided() {
		glog.V(5).Infof("Checking existing authentication info for '%v'...\n", c.username)

		config := configStore.Config

		for _, ctx := range config.Contexts {
			authInfo := config.AuthInfos[ctx.AuthInfo]
			clusterInfo := config.Clusters[ctx.Cluster]

			if ctx.AuthInfo == c.username && clusterInfo.Server == c.server && len(authInfo.Token) > 0 { // only token for now
				glog.V(5).Infof("Authentication exists for '%v' on '%v', trying to use it...\n", c.server, c.username)

				clientCfg.BearerToken = authInfo.Token

				if me, err := whoami(clientCfg); err == nil && usernameFromUser(me) == c.username {
					c.authSetupInfo.Username = usernameFromUser(me)
					c.authSetupInfo.FullName = me.FullName
					c.authSetupInfo.Token = authInfo.Token
					c.authSetupInfo.NewAuth = false
					return c.authSetupInfo, nil
				}

				glog.V(5).Infof("Token %v no longer valid for '%v', can't use it\n", authInfo.Token, c.username)
			}
		}

	}

	// at this point we know we need to log in again
	clientCfg.BearerToken = ""
	accessToken, err := tokencmd.RequestToken(clientCfg, c.reader, c.username, c.password)
	if err != nil {
		// certificate issue, prompt to user decide about connecting insecurely
		if isServerCertificateSignedByUnknownAuthority(err) {
			glog.Warning("The server uses a certificate signed by unknown authority. You can bypass the certificate check but it will make all connections insecure.")

			insecure := util.PromptForBool(os.Stdin, "Use insecure connections [y/N]? ")
			insecureFlag := c.cmd.Flags().Lookup("insecure-skip-tls-verify")

			if insecure && insecureFlag != nil {
				c.serverSetupInfo.Insecure = insecure
				insecureFlag.Value.Set(strconv.FormatBool(insecure))
				if err := c.cmd.Execute(); err != nil {
					glog.Fatal(err)
				}
				os.Exit(0)
			}
		}

		return nil, err
	}

	// last login was successful
	if me, err := whoami(clientCfg); err == nil {
		c.authSetupInfo.Username = usernameFromUser(me)
		c.authSetupInfo.FullName = me.FullName
		c.authSetupInfo.Token = accessToken
		c.authSetupInfo.NewAuth = true
		return c.authSetupInfo, nil
	}

	return nil, err
}

func (c OscClientSetup) DetermineProjectInfo() (*ProjectSetupInfo, error) {
	clientCfg, err := c.factory.OpenShiftClientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	oClient, err := client.New(clientCfg)
	if err != nil {
		return nil, err
	}

	projects, err := oClient.Projects().List(labels.Everything(), labels.Everything())
	if err != nil {
		return nil, err
	}

	projectsItems := projects.Items
	projectsLength := len(projectsItems)

	switch projectsLength {
	case 0:
		return nil, fmt.Errorf(`You don't have any project.
Use the 'openshift ex new-project <project-name>' command to create a new project.`) // TODO parameterize cmd name

	case 1:
		currentProject := projectsItems[0]

		c.projectSetupInfo.Projects = []string{currentProject.Name}
		c.projectSetupInfo.ProjectInUse = currentProject.Name

		return c.projectSetupInfo, nil
	}

	c.projectSetupInfo.Projects = make([]string, projectsLength)

	for i, project := range projectsItems {
		c.projectSetupInfo.Projects[i] = project.Name
	}

	current, err := c.factory.OpenShiftClientConfig.Namespace()
	if err != nil {
		return nil, err
	}
	c.projectSetupInfo.ProjectInUse = current

	return c.projectSetupInfo, nil
}

func (c OscClientSetup) MergeConfig() error {
	if c.serverSetupInfo == nil || c.authSetupInfo == nil || c.projectSetupInfo == nil {
		return fmt.Errorf("Insufficient data to merge configuration.")
	}

	clientCfg, err := c.factory.OpenShiftClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	configStore, err := config.GetConfigFromDefaultLocations(clientCfg, c.cmd)
	if err != nil {
		if _, isNotFound := err.(*config.ConfigStoreNotFoundError); isNotFound {
			if configStore, err = config.CreateNewEmptyConfig(); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	err = config.UpdateConfigFile(c.authSetupInfo.Username, c.authSetupInfo.Token, c.factory.OpenShiftClientConfig.KClientConfig, configStore)
	return err
}

func (c *OscClientSetup) usernameProvided() bool {
	return len(c.username) > 0
}

func (c *OscClientSetup) passwordProvided() bool {
	return len(c.password) > 0
}

func (c *OscClientSetup) serverProvided() bool {
	return len(c.server) > 0
}

func (c *OscClientSetup) contextProvided() bool {
	return len(c.context) > 0
}

func usernameFromUser(user *api.User) string {
	return strings.Split(user.Name, ":")[1]
}

func isOpenShiftNotConfigured(err error) bool {
	if err != nil {
		err, ok := err.(clientcmd.ClientError)
		return ok && err.IsNotConfigured()
	}
	return false
}

func isServerCertificateSignedByUnknownAuthority(err error) bool {
	if err != nil {
		err, ok := err.(clientcmd.ClientError)
		return ok && err.IsCertificateAuthorityUnknown()
	}
	return false
}

func whoami(clientCfg *kclient.Config) (*api.User, error) {
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
