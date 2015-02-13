/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clientcmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"

	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	clientcmdlatest "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
)

const (
	RecommendedConfigPathFlag   = "kubeconfig"
	RecommendedConfigPathEnvVar = "KUBECONFIG"
)

// ClientConfigLoadingRules is a collection that allows multiple sets of loading rules
type ClientConfigLoadingRules struct {
	Rules []ClientConfigLoadingRule
}

// ClientConfigLoadingRule is a struct that calls our specific locations that are used for merging together a Config
type ClientConfigLoadingRule struct {
	CommandLinePath      string
	EnvVarPath           string
	CurrentDirectoryPath string
	HomeDirectoryPath    string
}

// NewClientConfigLoadingRules returns a ClientConfigLoadingRules object with default fields filled in.  You are not required to
// use this constructor
func NewClientConfigLoadingRules() *ClientConfigLoadingRules {
	return &ClientConfigLoadingRules{
		Rules: []ClientConfigLoadingRule{
			ClientConfigLoadingRule{
				CurrentDirectoryPath: ".kubeconfig",
				HomeDirectoryPath:    os.Getenv("HOME") + "/.kube/.kubeconfig",
			},
		},
	}
}

// Returns the first loading rule, most use cases
func (rules *ClientConfigLoadingRules) Default() *ClientConfigLoadingRule {
	if len(rules.Rules) > 0 {
		return &rules.Rules[0]
	}
	return nil
}

func (rules *ClientConfigLoadingRules) AppendRule(commandLinePath string, envVarPath string, currentDirectoryPath string, homeDirectoryPath string) {
	rules.Rules = append(rules.Rules, ClientConfigLoadingRule{
		CommandLinePath:      commandLinePath,
		EnvVarPath:           envVarPath,
		CurrentDirectoryPath: currentDirectoryPath,
		HomeDirectoryPath:    homeDirectoryPath,
	})
}

func (rules *ClientConfigLoadingRules) PrependRule(commandLinePath string, envVarPath string, currentDirectoryPath string, homeDirectoryPath string) {
	rules.Rules = append([]ClientConfigLoadingRule{
		ClientConfigLoadingRule{
			CommandLinePath:      commandLinePath,
			EnvVarPath:           envVarPath,
			CurrentDirectoryPath: currentDirectoryPath,
			HomeDirectoryPath:    homeDirectoryPath,
		},
	}, rules.Rules...)
}

// Load takes the loading rules and merges together a Config object based on following order.
//   1.  CommandLinePath
//   2.  EnvVarPath
//   3.  CurrentDirectoryPath
//   4.  HomeDirectoryPath
// A missing CommandLinePath file produces an error. Empty filenames or other missing files are ignored.
// Read errors or files with non-deserializable content produce errors.
// The first file to set a particular map key wins and map key's value is never changed.
// BUT, if you set a struct value that is NOT contained inside of map, the value WILL be changed.
// This results in some odd looking logic to merge in one direction, merge in the other, and then merge the two.
// It also means that if two files specify a "red-user", only values from the first file's red-user are used.  Even
// non-conflicting entries from the second file's "red-user" are discarded.
// Relative paths inside of the .kubeconfig files are resolved against the .kubeconfig file's parent folder
// and only absolute file paths are returned.
func (rules *ClientConfigLoadingRules) Load() (*clientcmdapi.Config, error) {

	errlist := []error{}

	// Make sure a file we were explicitly told to use exists
	if len(rules.CommandLinePath) > 0 {
		if _, err := os.Stat(rules.CommandLinePath); os.IsNotExist(err) {
			errlist = append(errlist, fmt.Errorf("The config file %v does not exist", rules.CommandLinePath))
		}
	}

	kubeConfigFiles := []string{}
	for _, rule := range rules.Rules {
		kubeConfigFiles = append(kubeConfigFiles, rule.CommandLinePath)
		kubeConfigFiles = append(kubeConfigFiles, rule.EnvVarPath)
		kubeConfigFiles = append(kubeConfigFiles, rule.CurrentDirectoryPath)
		kubeConfigFiles = append(kubeConfigFiles, rule.HomeDirectoryPath)
	}

	// first merge all of our maps
	mapConfig := clientcmdapi.NewConfig()
	for _, file := range kubeConfigFiles {
		if err := mergeConfigWithFile(mapConfig, file); err != nil {
			errlist = append(errlist, err)
		}
		if err := resolveLocalPaths(file, mapConfig); err != nil {
			errlist = append(errlist, err)
		}
	}

	// merge all of the struct values in the reverse order so that priority is given correctly
	// errors are not added to the list the second time
	nonMapConfig := clientcmdapi.NewConfig()
	for i := len(kubeConfigFiles) - 1; i >= 0; i-- {
		file := kubeConfigFiles[i]
		mergeConfigWithFile(nonMapConfig, file)
		resolveLocalPaths(file, nonMapConfig)
	}

	// since values are overwritten, but maps values are not, we can merge the non-map config on top of the map config and
	// get the values we expect.
	config := clientcmdapi.NewConfig()
	mergo.Merge(config, mapConfig)
	mergo.Merge(config, nonMapConfig)

	return config, errors.NewAggregate(errlist)
}

func mergeConfigWithFile(startingConfig *clientcmdapi.Config, filename string) error {
	if len(filename) == 0 {
		// no work to do
		return nil
	}

	config, err := LoadFromFile(filename)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("Error loading config file \"%s\": %v", filename, err)
	}

	mergo.Merge(startingConfig, config)

	return nil
}

// resolveLocalPaths resolves all relative paths in the config object with respect to the parent directory of the filename
// this cannot be done directly inside of LoadFromFile because doing so there would make it impossible to load a file without
// modification of its contents.
func resolveLocalPaths(filename string, config *clientcmdapi.Config) error {
	if len(filename) == 0 {
		return nil
	}

	configDir, err := filepath.Abs(filepath.Dir(filename))
	if err != nil {
		return fmt.Errorf("Could not determine the absolute path of config file %s: %v", filename, err)
	}

	resolvedClusters := make(map[string]clientcmdapi.Cluster)
	for key, cluster := range config.Clusters {
		cluster.CertificateAuthority = resolveLocalPath(configDir, cluster.CertificateAuthority)
		resolvedClusters[key] = cluster
	}
	config.Clusters = resolvedClusters

	resolvedAuthInfos := make(map[string]clientcmdapi.AuthInfo)
	for key, authInfo := range config.AuthInfos {
		authInfo.AuthPath = resolveLocalPath(configDir, authInfo.AuthPath)
		authInfo.ClientCertificate = resolveLocalPath(configDir, authInfo.ClientCertificate)
		authInfo.ClientKey = resolveLocalPath(configDir, authInfo.ClientKey)
		resolvedAuthInfos[key] = authInfo
	}
	config.AuthInfos = resolvedAuthInfos

	return nil
}

// resolveLocalPath makes the path absolute with respect to the startingDir
func resolveLocalPath(startingDir, path string) string {
	if len(path) == 0 {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(startingDir, path)
}

// LoadFromFile takes a filename and deserializes the contents into Config object
func LoadFromFile(filename string) (*clientcmdapi.Config, error) {
	kubeconfigBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return Load(kubeconfigBytes)
}

// Load takes a byte slice and deserializes the contents into Config object.
// Encapsulates deserialization without assuming the source is a file.
func Load(data []byte) (*clientcmdapi.Config, error) {
	config := &clientcmdapi.Config{}
	if err := clientcmdlatest.Codec.DecodeInto(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

// WriteToFile serializes the config to yaml and writes it out to a file.  If not present, it creates the file with the mode 0600.  If it is present
// it stomps the contents
func WriteToFile(config clientcmdapi.Config, filename string) error {
	content, err := Write(config)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filename, content, 0600); err != nil {
		return err
	}
	return nil
}

// Write serializes the config to yaml.
// Encapsulates serialization without assuming the destination is a file.
func Write(config clientcmdapi.Config) ([]byte, error) {
	json, err := clientcmdlatest.Codec.Encode(&config)
	if err != nil {
		return nil, err
	}
	content, err := yaml.JSONToYAML(json)
	if err != nil {
		return nil, err
	}
	return content, nil
}
