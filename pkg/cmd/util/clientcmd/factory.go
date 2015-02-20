package clientcmd

import (
	"fmt"
	"net/http"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
)

const DefaultClusterURL = "https://localhost:8443"

// NewFactory creates a default Factory for commands that should share identical server
// connection behavior. Most commands should use this method to get a factory.
func New(flags *pflag.FlagSet) *Factory {
	// Override global default to https and port 8443
	clientcmd.DefaultCluster.Server = ""

	// TODO: there should be two client configs, one for OpenShift, and one for Kubernetes
	clientConfig := DefaultClientConfig(flags)
	f := NewFactory(clientConfig)
	f.BindFlags(flags)

	return f
}

// Copy of kubectl/cmd/DefaultClientConfig, using NewNonInteractiveDeferredLoadingClientConfig
func DefaultClientConfig(flags *pflag.FlagSet) clientcmd.ClientConfig {
	loadingRules := config.NewOpenShiftClientConfigLoadingRules()

	flags.StringVar(&loadingRules.Default().CommandLinePath, config.OpenShiftConfigFlagName, "", "Path to the config file to use for CLI requests.")

	overrides := &clientcmd.ConfigOverrides{}
	overrideFlags := clientcmd.RecommendedConfigOverrideFlags("")
	overrideFlags.ContextOverrideFlags.NamespaceShort = "n"
	clientcmd.BindOverrideFlags(overrides, flags, overrideFlags)

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	return clientConfig
}

// Factory provides common options for OpenShift commands
type Factory struct {
	*kubecmd.Factory
	OpenShiftClientConfig ClientConfig
}

// intercept errors that need special treatment for end-users (e.g. no config file present)
type ClientConfig struct {
	KClientConfig clientcmd.ClientConfig
}

func (cfg *ClientConfig) ClientConfig() (*kclient.Config, error) {
	c, err := cfg.KClientConfig.ClientConfig()
	err = DecorateErrors(err)
	return c, err
}

func (cfg *ClientConfig) RawConfig() (clientcmdapi.Config, error) {
	c, err := cfg.KClientConfig.RawConfig()
	err = DecorateErrors(err)
	return c, err
}

func (cfg *ClientConfig) Namespace() (string, error) {
	n, err := cfg.KClientConfig.Namespace()
	err = DecorateErrors(err)
	return n, err
}

// NewFactory creates an object that holds common methods across all OpenShift commands
func NewFactory(clientConfig clientcmd.ClientConfig) *Factory {
	mapper := ShortcutExpander{kubectl.ShortcutExpander{latest.RESTMapper}}

	w := &Factory{kubecmd.NewFactory(clientConfig), ClientConfig{clientConfig}}

	w.Object = func(cmd *cobra.Command) (meta.RESTMapper, runtime.ObjectTyper) {
		return mapper, api.Scheme
	}

	w.RESTClient = func(cmd *cobra.Command, mapping *meta.RESTMapping) (resource.RESTClient, error) {
		oClient, kClient, err := w.Clients(cmd)
		if err != nil {
			return nil, fmt.Errorf("unable to create client %s: %v", mapping.Kind, err)
		}

		if latest.OriginKind(mapping.Kind, mapping.APIVersion) {
			return oClient.RESTClient, nil
		} else {
			return kClient.RESTClient, nil
		}
	}

	w.Describer = func(cmd *cobra.Command, mapping *meta.RESTMapping) (kubectl.Describer, error) {
		oClient, kClient, err := w.Clients(cmd)
		if err != nil {
			return nil, fmt.Errorf("unable to create client %s: %v", mapping.Kind, err)
		}

		if latest.OriginKind(mapping.Kind, mapping.APIVersion) {
			describer, ok := describe.DescriberFor(mapping.Kind, oClient, kClient, "")
			if !ok {
				return nil, fmt.Errorf("no description has been implemented for %q", mapping.Kind)
			}
			return describer, nil
		} else {
			describer, ok := kubectl.DescriberFor(mapping.Kind, kClient)
			if !ok {
				return nil, fmt.Errorf("no description has been implemented for %q", mapping.Kind)
			}
			return describer, nil
		}
	}

	w.Printer = func(cmd *cobra.Command, mapping *meta.RESTMapping, noHeaders bool) (kubectl.ResourcePrinter, error) {
		return describe.NewHumanReadablePrinter(noHeaders), nil
	}

	w.DefaultNamespace = func(cmd *cobra.Command) (string, error) {
		return w.OpenShiftClientConfig.Namespace()
	}

	return w
}

// Clients returns an OpenShift and Kubernetes client.
func (f *Factory) Clients(cmd *cobra.Command) (*client.Client, *kclient.Client, error) {
	cfg, err := f.OpenShiftClientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	transport, err := kclient.TransportFor(cfg)
	if err != nil {
		return nil, nil, err
	}
	httpClient := &http.Client{
		Transport: transport,
	}

	oClient, err := client.New(cfg)
	if err != nil {
		return nil, nil, err
	}
	kClient, err := kclient.New(cfg)
	if err != nil {
		return nil, nil, err
	}

	oClient.Client = &statusHandlerClient{httpClient}
	kClient.Client = &statusHandlerClient{httpClient}

	return oClient, kClient, nil
}

// ShortcutExpander is a RESTMapper that can be used for OpenShift resources.
type ShortcutExpander struct {
	meta.RESTMapper
}

// VersionAndKindForResource implements meta.RESTMapper. It expands the resource first, then invokes the wrapped
// mapper.
func (e ShortcutExpander) VersionAndKindForResource(resource string) (defaultVersion, kind string, err error) {
	resource = expandResourceShortcut(resource)
	return e.RESTMapper.VersionAndKindForResource(resource)
}

// expandResourceShortcut will return the expanded version of resource
// (something that a pkg/api/meta.RESTMapper can understand), if it is
// indeed a shortcut. Otherwise, will return resource unmodified.
func expandResourceShortcut(resource string) string {
	shortForms := map[string]string{
		"dc": "deploymentConfigs",
		"bc": "buildConfigs",
	}
	if expanded, ok := shortForms[resource]; ok {
		return expanded
	}
	return resource
}
