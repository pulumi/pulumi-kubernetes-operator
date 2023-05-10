package cluster

import (
	"encoding/base32"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/cluster"

	. "github.com/onsi/gomega"
)

// KindVersion is a specific Kind version associated with a Kubernetes minor version.
type KindVersion string

const (
	Kind1_24 KindVersion = "kindest/node:v1.24.0@sha256:0866296e693efe1fed79d5e6c7af8df71fc73ae45e3679af05342239cdc5bc8e"

	// Kubeconfig is the filename of the KUBECONFIG file.
	Kubeconfig = "KUBECONFIG"

	// maxCreateTries is the maximum number of times to try to create a Kind cluster.
	maxCreateTries = 6
)

// restConfig returns a rest.Config for the specified kubeconfig path.
func restConfig(kubeconfigPath string) *rest.Config {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	Expect(err).ToNot(HaveOccurred())
	return config
}

// newKindCluster creates a new Kind cluster for use in testing with the specified name.
func NewKindCluster(name string) (*rest.Config, func()) {
	// Create a new tmpdir for the test.
	tmpDir, err := ioutil.TempDir("", "pulumi-test-kind")
	Expect(err).ToNot(HaveOccurred())
	KubeconfigPath := filepath.Join(tmpDir, Kubeconfig)

	p := cluster.NewProvider()

	name += "-" + randString()
	shutdown, err := createKindCluster(p, name, KubeconfigPath, Kind1_24)
	Expect(err).To(Succeed())

	return restConfig(KubeconfigPath), shutdown

}

func randString() string {
	rand.Seed(time.Now().UnixNano())
	c := 10
	b := make([]byte, c)
	rand.Read(b)
	length := 6
	return strings.ToLower(base32.StdEncoding.EncodeToString(b)[:length])
}

func createKindCluster(p *cluster.Provider, name, kcfgPath string, version KindVersion) (func(), error) {
	var err error
	for i := 0; i < maxCreateTries; i++ {
		err = p.Create(name,
			cluster.CreateWithKubeconfigPath(kcfgPath),
			cluster.CreateWithNodeImage(string(version)),
			cluster.CreateWithWaitForReady(10*time.Minute),
		)
		if err == nil {
			return func() {
				deleteKindCluster(p, name, kcfgPath)
			}, nil
		}
	}

	// We failed to create the cluster maxKindTries times, so fail out.
	return nil, err
}

func deleteKindCluster(p *cluster.Provider, name, kcfgPath string) error {
	return p.Delete(name, kcfgPath)
}
