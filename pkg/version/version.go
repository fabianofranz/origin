package version

import (
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

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

type tmpFile struct {
	*os.File
}

func (t *tmpFile) Delete() error {
	t.Close()
	return os.Remove(t.Name())
}

// NewVersionCommand creates a command for displaying the version of this binary
func NewVersionCommand(basename string, printEtcdVersion bool) *cobra.Command {
	baseUpdaterURL := "http://ocupdater-ffranz.rhcloud.com/updates/repository"
	pubKey, _ := hex.DecodeString("c9415ba32c55b3242f6ec5db32be978b3c947654e74dca8f3fc804664f514ec7")

	update := false
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version",
		Run: func(c *cobra.Command, args []string) {
			switch {
			case update:
				localStore := updater.MemoryLocalStore()
				remoteStore, err := updater.HTTPRemoteStore(baseUpdaterURL, &updater.HTTPRemoteOptions{})
				if err != nil {
					fmt.Println("Error fetching remote store: " + err.Error())
					return
				}
				client := updater.NewClient(localStore, remoteStore)
				rootKey := &data.Key{
					"ed25519",
					data.KeyValue{
						[]byte(pubKey),
						nil,
					},
				}
				if err := client.Init([]*data.Key{rootKey}, 1); err != nil {
					fmt.Println("Error initializing client: " + err.Error())
					return
				}

				fmt.Print("Checking for updates... ")
				if _, err := client.Update(); err != nil {
					if updater.IsLatestSnapshot(err) {
						fmt.Println("already up-to-date!")
						return
					}
					fmt.Println("\nError checking updates: " + err.Error())
					return
				}
				fmt.Println("update available")

				//fmt.Println("Listing targets...")
				targets, err := client.Targets()
				if err != nil {
					fmt.Println("Error listing targets: " + err.Error())
					return
				}
				for path, meta := range targets {
					fmt.Printf("%v size: %v bytes\n", path, meta.Length)
				}

				fmt.Print("Downloading... ")
				tempFile, err := ioutil.TempFile("", "oc")
				if err != nil {
					fmt.Println("\nError creating file: " + err.Error())
					return
				}
				tmp := tmpFile{tempFile}
				if err := client.Download("/oc", &tmp); err != nil {
					fmt.Println("\nError downloading update: ", err.Error())
				}
				fmt.Println("downloaded")
				if _, err := tmp.Seek(0, os.SEEK_SET); err != nil {
					fmt.Println("Error creating file: " + err.Error())
				}

				dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
				out, err := os.Create(dir + "/oc")
				if err != nil {
					//fmt.Println("Error creating file: " + err.Error())
					return
				}
				if _, err = io.Copy(out, tempFile); err != nil {
					//fmt.Println("Error copying file: " + err.Error())
					return
				}
				defer tmp.Delete()
				defer out.Close()
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
