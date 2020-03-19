package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	rspb "k8s.io/helm/pkg/proto/hapi/release"

	// Enable usage of the following providers
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	_ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
)

type ListOptions struct {
	ReleaseName     string
	TillerNamespace string
	TillerLabel     string
}

type ReleaseData struct {
	Name      string
	Revision  int32
	Updated   string
	Status    string
	Chart     string
	Namespace string
	Time      time.Time
	Manifest  string
}

// ListReleases lists all releases according to provided options
func ListReleases(o ListOptions) ([]ReleaseData, error) {
	return ListReleasesWithKubeConfig(o, "", "")
}

// ListReleasesWithKubeConfig lists all releases according to provided options
func ListReleasesWithKubeConfig(o ListOptions, kubeConfigFile, context string) ([]ReleaseData, error) {
	if o.TillerNamespace == "" {
		o.TillerNamespace = "kube-system"
	}
	if o.TillerLabel == "" {
		o.TillerLabel = "OWNER=TILLER"
	}
	if o.ReleaseName != "" {
		o.TillerLabel += fmt.Sprintf(",NAME=%s", o.ReleaseName)
	}
	clientSet := GetClientSetWithKubeConfig(kubeConfigFile, context)
	var releasesData []ReleaseData
	storage := GetTillerStorageWithKubeConfig(o.TillerNamespace, kubeConfigFile, context)
	switch storage {
	case "secrets":
		secrets, err := clientSet.CoreV1().Secrets(o.TillerNamespace).List(metav1.ListOptions{
			LabelSelector: o.TillerLabel,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range secrets.Items {
			releaseData := GetReleaseData((string)(item.Data["release"]))
			if releaseData == nil {
				continue
			}
			releasesData = append(releasesData, *releaseData)
		}
	case "configmaps":
		configMaps, err := clientSet.CoreV1().ConfigMaps(o.TillerNamespace).List(metav1.ListOptions{
			LabelSelector: o.TillerLabel,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range configMaps.Items {
			releaseData := GetReleaseData(item.Data["release"])
			if releaseData == nil {
				continue
			}
			releasesData = append(releasesData, *releaseData)
		}
	}

	return releasesData, nil
}

type ListReleaseNamesInNamespaceOptions struct {
	Namespace       string
	TillerNamespace string
}

// ListReleaseNamesInNamespace returns a string list of all releases in a provided namespace
func ListReleaseNamesInNamespace(o ListReleaseNamesInNamespaceOptions) (string, error) {
	return ListReleaseNamesInNamespaceWithKubeConfig(o, "", "")
}

// ListReleaseNamesInNamespaceWithKubeConfig returns a string list of all releases in a provided namespace
func ListReleaseNamesInNamespaceWithKubeConfig(o ListReleaseNamesInNamespaceOptions, kubeConfigFile, context string) (string, error) {
	releases, err := ListReleasesWithKubeConfig(ListOptions{
		TillerNamespace: o.TillerNamespace,
	}, kubeConfigFile, context)
	if err != nil {
		return "", err
	}
	uniqReleases := make(map[string]string)
	for _, r := range releases {
		if r.Namespace != o.Namespace {
			continue
		}
		uniqReleases[r.Name] = ""
	}
	var inReleases string
	for k := range uniqReleases {
		inReleases += k
		inReleases += ","
	}
	return strings.TrimRight(inReleases, ","), nil
}

// GetReleaseData returns a decoded structed release data
func GetReleaseData(itemReleaseData string) *ReleaseData {
	data, _ := DecodeRelease(itemReleaseData)
	deployTime := time.Unix(data.Info.LastDeployed.Seconds, 0)
	chartMeta := data.GetChart().Metadata

	releaseData := ReleaseData{
		Name:      data.Name,
		Revision:  data.Version,
		Updated:   deployTime.Format("Mon Jan _2 15:04:05 2006"),
		Status:    data.GetInfo().Status.Code.String(),
		Chart:     chartMeta.Name,
		Namespace: data.Namespace,
		Time:      deployTime,
		Manifest:  data.Manifest,
	}
	return &releaseData
}

// DecodeRelease decodes release data from a tiller resource (configmap/secret)
func DecodeRelease(data string) (*rspb.Release, error) {
	// base64 decode string
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	// For backwards compatibility with releases that were stored before
	// compression was introduced we skip decompression if the
	// gzip magic header is not found
	if bytes.Equal(b[0:3], []byte{0x1f, 0x8b, 0x08}) {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		b2, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}
		b = b2
	}

	var rls rspb.Release
	// unmarshal protobuf bytes
	if err := proto.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}

// GetClientSet returns a kubernetes ClientSet
func GetClientSet() *kubernetes.Clientset {
	return GetClientSetWithKubeConfig("", "")
}

// GetClientSetWithKubeConfig returns a kubernetes ClientSet
func GetClientSetWithKubeConfig(kubeConfigFile, context string) *kubernetes.Clientset {
	var kubeConfigFiles []string
	if kubeConfigFile != "" {
		kubeConfigFiles = append(kubeConfigFiles, kubeConfigFile)
	} else if kubeConfigPath := os.Getenv("KUBECONFIG"); kubeConfigPath != "" {
		// The KUBECONFIG environment variable holds a list of kubeconfig files.
		// For Linux and Mac, the list is colon-delimited. For Windows, the list
		// is semicolon-delimited. Ref:
		// https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/#the-kubeconfig-environment-variable
		var separator string
		if runtime.GOOS == "windows" {
			separator = ";"
		} else {
			separator = ":"
		}
		kubeConfigFiles = strings.Split(kubeConfigPath, separator)
	} else {
		kubeConfigFiles = append(kubeConfigFiles, filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	}

	config, err := buildConfigFromFlags(context, kubeConfigFiles)
	if err != nil {
		log.Fatal(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err.Error())
	}

	return clientset
}

func buildConfigFromFlags(context string, kubeConfigFiles []string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{Precedence: kubeConfigFiles},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}

// GetTillerStorage returns the storage type of tiller (configmaps/secrets)
func GetTillerStorage(tillerNamespace string) string {
	return GetTillerStorageWithKubeConfig(tillerNamespace, "", "")
}

// GetTillerStorageWithKubeConfig returns the storage type of tiller (configmaps/secrets)
func GetTillerStorageWithKubeConfig(tillerNamespace, kubeConfigFile, context string) string {
	clientset := GetClientSetWithKubeConfig(kubeConfigFile, context)
	coreV1 := clientset.CoreV1()
	listOptions := metav1.ListOptions{
		LabelSelector: "name=tiller",
	}
	pods, err := coreV1.Pods(tillerNamespace).List(listOptions)
	if err != nil {
		log.Fatal(err)
	}

	if len(pods.Items) == 0 {
		log.Fatal("Found 0 tiller pods")
	}

	storage := "configmaps"
	container := pods.Items[0].Spec.Containers[0]
	for _, c := range container.Command {
		if strings.Contains(c, "secret") {
			storage = "secrets"
		}
	}
	for _, a := range container.Args {
		if strings.Contains(a, "storage=secret") {
			storage = "secrets"
		}
	}

	return storage
}

// Execute executes a command
func Execute(cmd []string) []byte {
	binary := cmd[0]
	_, err := exec.LookPath(binary)
	if err != nil {
		log.Fatal(err)
	}

	output, err := exec.Command(binary, cmd[1:]...).Output()
	if err != nil {
		log.Println("Error: command execution failed:", cmd)
	}

	return output
}

// ExecuteCombined executes a command and resturns the combined output
func ExecuteCombined(cmd []string) []byte {
	binary := cmd[0]
	_, err := exec.LookPath(binary)
	if err != nil {
		log.Fatal(err)
	}

	output, err := exec.Command(binary, cmd[1:]...).CombinedOutput()
	if err != nil {
		log.Println("Error: command execution failed:", cmd)
		log.Fatal(string(output))
	}

	return output
}
