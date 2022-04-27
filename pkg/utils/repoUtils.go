package utils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dell/csm-operator/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cmcorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	cmmetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
	//"k8s.io/apimachinery/pkg/api/errors"
)

type InstallConfig struct {
	Listofconfigmapnames  []string
	UpgradePathMinVersion string
	UpgradePathToVersion  string
}

// download tgx file from remote repo
// do security checks to make sure  url is authentic
func Download(ctx context.Context, repository string, client http.Client) (*bytes.Buffer, error) {
	//repoURL := "https://amaas-eos-mw1.cec.lab.emc.com:5036/artifactory/csi-driver-helm-virtual/powerscale-v2.2.0.tgz"

	log := logger.GetLogger(ctx)

	u, err := url.Parse(repository)
	if err != nil {
		return nil, err
	}

	var isStringAlphabetic = regexp.MustCompile(`^[a-zA-Z0-9-.]+$`).MatchString
	if !u.IsAbs() || u.Scheme != "https" || len(u.Query()) > 0 || !isStringAlphabetic(u.Hostname()) {
		return nil, fmt.Errorf("bad url issue : %s", u.String())
	}

	if strings.Contains(u.String(), "@") ||
		!strings.HasPrefix(u.String(), "https://") ||
		!strings.HasSuffix(u.String(), "tgz") ||
		strings.Contains(u.String(), "\\") ||
		strings.Contains(u.String(), "#") ||
		strings.Contains(u.Path, "//") ||
		strings.Contains(u.Path, "/.") {
		fmt.Printf("bad url %s", u.String())
		return nil, fmt.Errorf("bad url issue : %s", u.String())
	}

	fmt.Println("download from ", u.String())

	pluginData, err := Get(u, client)
	if err != nil {
		log.Errorw("Not able to download file", "Error", err.Error())
		return nil, err
	}

	return pluginData, nil
}

// Get similar to scalio Get call
func Get(u *url.URL, client http.Client) (*bytes.Buffer, error) {
	ctx := context.Background()
	timeout := time.Second * 5

	transport := &http.Transport{
		DisableCompression: true,
		Proxy:              http.ProxyFromEnvironment,
	}

	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	client.Transport = transport
	client.Timeout = timeout

	// Get the URL
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	// use your own user and personal token

	// expected 401 unauthorized if you use invalid user /pswd

	// ========================================
	username := "murala7"
	password := "AKCp8krKqQCTgALPTEW4SSjBvmnKx2Ym7goZhFGnqCe2aQULvo5udHieigZGLRG5xvRYmaasP"
	// ========================================

	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s : %s", u, resp.Status)
	}
	fmt.Printf("http get Status code : %d", resp.StatusCode)
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, resp.Body)
	return buf, err

}

type data struct {
	yamls map[string]string
}

// Extract - tar unzip tgz file and create config map from file contents
func ExtractandCreateMap(ctx context.Context, buffer *bytes.Buffer, nameofMap string, k8sClient kubernetes.Interface) ([]string, error) {

	//log := logger.GetLogger(ctx)
	uncompressedStream, err := gzip.NewReader(buffer)
	if err != nil {
		return nil, err
	}

	tarReader := tar.NewReader(uncompressedStream)
	configMapData := make(map[string]data, 0)
	configMapName := make([]string, 0)
	metaDataMap := make(map[string]string, 0)

	ns := OperatorNamespace

	cmName := ""
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		fmt.Printf("** create configmap with name %s and namespace %s\n", cmName, ns)
		tarFile := filepath.Base(header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			cmName = strings.ReplaceAll(header.Name, "/", "")
			fmt.Printf("**found new directory from tar %s and namespace %s\n", cmName, ns)
			d := new(data)
			d.yamls = make(map[string]string)
			configMapData[cmName] = *d
			continue
		case tar.TypeReg:
			if strings.Contains(header.Name, nameofMap) {
				// extract the file and get list of configmap names
				var b bytes.Buffer
				b.ReadFrom(tarReader)
				key := filepath.Base(header.Name)
				metaDataMap[key] = b.String()
				_ = CreateMap(ctx, nameofMap, ns, metaDataMap, k8sClient)
				configMapName, err = ConfigmapReader(ctx, k8sClient, nameofMap, ns, configMapName)
				if err != nil {
					return nil, err
				}
				break
			}
			if strings.Contains(header.Name, cmName) {
				// save to configmap
				var b bytes.Buffer
				b.ReadFrom(tarReader)
				if tarFile != "" {
					configMapData[cmName].yamls[tarFile] = b.String()
				}
			}
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		default:
			return nil, fmt.Errorf("unknown type: %b in %s", header.Typeflag, header.Name)
		}
	}

	for _, cmName := range configMapName {
		_ = CreateMap(ctx, cmName, ns, configMapData[cmName].yamls, k8sClient)
	}

	return configMapName, nil
}

// CreateMap - create configmap if needed else update existing map
func CreateMap(ctx context.Context, name string, ns string, configMapData map[string]string, k8sClient kubernetes.Interface) error {

	log := logger.GetLogger(ctx)
	kindName := "ConfigMap"
	apiVersion := "v1"
	immutable := true
	configMap := &cmcorev1.ConfigMapApplyConfiguration{
		TypeMetaApplyConfiguration: cmmetav1.TypeMetaApplyConfiguration{
			Kind:       &kindName,
			APIVersion: &apiVersion,
		},
		ObjectMetaApplyConfiguration: &cmmetav1.ObjectMetaApplyConfiguration{
			Name:      &name,
			Namespace: &ns,
		},
		Data:      configMapData,
		Immutable: &immutable,
	}

	opts := metav1.ApplyOptions{FieldManager: "application/apply-patch"}
	cm, err := k8sClient.CoreV1().ConfigMaps(ns).Apply(ctx, configMap, opts)
	if err != nil {
		log.Errorw("Failed to Apply Map", "error", err.Error())
		return err
	}
	fmt.Println("found configmap ", cm.Name)

	return nil

}

func CheckMaps(ctx context.Context, configName []string, ns string, nameofMap string, k8sClient kubernetes.Interface) (bool, error) {

	var cm *corev1.ConfigMap
	var err error
	isFound := false
	if cm, err = k8sClient.CoreV1().ConfigMaps(ns).Get(ctx, nameofMap, metav1.GetOptions{}); err != nil {

		fmt.Println("new configmap needed", err.Error())

		fmt.Printf("create new configmap %+v", cm.Name)
		return isFound, nil
	}
	configName, err = ConfigmapReader(ctx, k8sClient, nameofMap, ns, configName)
	for _, name := range configName {
		if cm, err := k8sClient.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{}); err != nil {

			fmt.Printf("Failed to apply maps from yaml file and need to create new configmap %+v", cm.Name)
			return isFound, err
		}
	}
	isFound = true
	return isFound, nil
}

func ConfigmapReader(ctx context.Context, k8sClient kubernetes.Interface, nameofMap string, ns string, configMapName []string) ([]string, error) {

	var cm *corev1.ConfigMap
	log := logger.GetLogger(ctx)
	var err error
	if cm, err = k8sClient.CoreV1().ConfigMaps(ns).Get(ctx, nameofMap, metav1.GetOptions{}); err != nil {
		fmt.Println("configmap not created", err.Error())
	}
	// read name of map and make a list of configname
	configMapYaml := cm.Data[nameofMap+".yaml"]
	var install InstallConfig
	err = yaml.Unmarshal([]byte(configMapYaml), &install)
	if err != nil {
		log.Errorw("Error reading yaml", "error", err.Error())
		return nil, err
	}
	configMapName = install.Listofconfigmapnames
	return configMapName, nil

}
