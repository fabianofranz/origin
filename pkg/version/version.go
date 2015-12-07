package version

import (
	"fmt"

	etcdversion "github.com/coreos/etcd/version"
	updater "github.com/flynn/go-tuf/client"
	"github.com/flynn/go-tuf/data"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	kubeversion "k8s.io/kubernetes/pkg/version"
)

var (
	// commitFromGit is a constant representing the source version that
	// generated this build. It should be set during build via -ldflags.
	commitFromGit string
	// versionFromGit is a constant representing the version tag that
	// generated this build. It should be set during build via -ldflags.
	versionFromGit string
	// major version
	majorFromGit string
	// minor version
	minorFromGit string
)

// Info contains versioning information.
// TODO: Add []string of api versions supported? It's still unclear
// how we'll want to distribute that information.
type Info struct {
	Major      string `json:"major"`
	Minor      string `json:"minor"`
	GitCommit  string `json:"gitCommit"`
	GitVersion string `json:"gitVersion"`
}

// Get returns the overall codebase version. It's for detecting
// what code a binary was built from.
func Get() Info {
	return Info{
		Major:      majorFromGit,
		Minor:      minorFromGit,
		GitCommit:  commitFromGit,
		GitVersion: versionFromGit,
	}
}

// String returns info as a human-friendly version string.
func (info Info) String() string {
	version := info.GitVersion
	if version == "" {
		version = "unknown"
	}
	return version
}

// NewVersionCommand creates a command for displaying the version of this binary
func NewVersionCommand(basename string, printEtcdVersion bool) *cobra.Command {
	baseUpdaterURL := "http://ocupdater-ffranz.rhcloud.com/updates/repository"
	rootKeys := []*data.Key{
		&data.Key{
			"ed25519",
			data.KeyValue{
				[]byte("7b7e9101b855186da8c6072df197162c1f0fa641bcf29dc638a5e327002287ae"),
				[]byte{},
			},
		},
		&data.Key{
			"ed25519",
			data.KeyValue{
				[]byte("cf488bab850635f2e9993c3ed4668b4b433ffbb8f1aaf750f97d7a27c2a0c1a9"),
				[]byte{},
			},
		},
	}

	check := false
	update := false
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version",
		Run: func(c *cobra.Command, args []string) {
			switch {
			case check:
				localStore := updater.MemoryLocalStore()
				remoteStore, err := updater.HTTPRemoteStore(baseUpdaterURL, &updater.HTTPRemoteOptions{})
				if err != nil {
					fmt.Println("Error fetching remote store: " + err.Error())
					return
				}
				client := updater.NewClient(localStore, remoteStore)
				if err := client.Init(rootKeys, len(rootKeys)); err != nil {
					fmt.Println("Error initializing client: " + err.Error())
					return
				}
				files, err := client.Targets()
				if err != nil {
					fmt.Println("Error listing targets: " + err.Error())
					return
				}
				for _, file := range files {
					fmt.Println(file.HashAlgorithms())
				}
				return

			case update:
				return

			default:
				fmt.Printf("%s %v\n", basename, Get())
				fmt.Printf("kubernetes %v\n", kubeversion.Get())
				if printEtcdVersion {
					fmt.Printf("etcd %v\n", etcdversion.Version)
				}
			}
		},
	}
	cmd.Flags().BoolVar(&check, "check", check, "Check if there is a new version of this client available")
	cmd.Flags().BoolVar(&update, "update", update, "Auto-update this client if there is a new version available")
	return cmd
}

func init() {
	buildInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "openshift_build_info",
			Help: "A metric with a constant '1' value labeled by major, minor, git commit & git version from which OpenShift was built.",
		},
		[]string{"major", "minor", "gitCommit", "gitVersion"},
	)
	buildInfo.WithLabelValues(majorFromGit, minorFromGit, commitFromGit, versionFromGit).Set(1)

	prometheus.MustRegister(buildInfo)
}
