package utils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
	//"k8s.io/apimachinery/pkg/api/errors"
)

type InstallConfig struct {
	Listofconfigmapnames  string
	UpgradePathMinVersion string
	UpgradePathToVersion  string
}

// download tgx file from remote repo
// do security checks to make sure  url is authentic
func Download(repository string) (*bytes.Buffer, error) {
	//repoURL := "https://amaas-eos-mw1.cec.lab.emc.com:5036/artifactory/csi-driver-helm-virtual/powerscale-v2.2.0.tgz"

	u, err := url.Parse(repository)
	if strings.Contains(u.String(), "@") ||
		!strings.HasPrefix(u.String(), "https://") ||
		!strings.HasSuffix(u.String(), "tgz") ||
		strings.Contains(u.String(), "\\") ||
		strings.Contains(u.String(), "#") ||
		strings.Contains(u.Path, "//") ||
		strings.Contains(u.Path, "/.") {

		fmt.Printf("bad url %s", u.String())
		panic(err)
	}

	var isStringAlphabetic = regexp.MustCompile(`^[a-zA-Z0-9-.]+$`).MatchString

	if err != nil || !u.IsAbs() || u.Scheme != "https" || len(u.Query()) > 0 || !isStringAlphabetic(u.Hostname()) {
		fmt.Printf("bad url issue %+v", u)
		panic(err)
	}

	fmt.Println("download from ", u.String())

	pluginData, err := Get(u)
	if err != nil {
		panic(err)
	}

	return pluginData, nil
}

// Get similar to scalio Get call
func Get(u *url.URL) (*bytes.Buffer, error) {
	ctx := context.Background()
	timeout := time.Second * 5

	transport := &http.Transport{
		DisableCompression: true,
		Proxy:              http.ProxyFromEnvironment,
	}

	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

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

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, resp.Body)
	return buf, err

}

// Extract - tar unzip tgz file and create config map from file contents
func ExtractandCreateMap(buffer *bytes.Buffer, nameofMap string) ([]string, error) {
	uncompressedStream, err := gzip.NewReader(buffer)
	if err != nil {
		return nil, err
	}

	tarReader := tar.NewReader(uncompressedStream)

	var configMapData map[string]string
	configMapData = make(map[string]string, 0)
	configMapName := make([]string, 0)
	name := ""
	ns := "dell-csm-operator"
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if strings.Contains(header.Name, nameofMap) {
			// extract the file and get list of configmap names
			var b []byte
			//b.ReadFrom(tarReader)
			var install InstallConfig
			err = yaml.Unmarshal(b, &install)
			if err != nil {
				return nil, err
			}
			configMapName = strings.Split(install.Listofconfigmapnames, ",")
			break
		}
	}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		//add for loop for each configmapname
		for _, cmName := range configMapName {
			key := ""
			// header.name = /a/b/c/foo.yaml
			name = cmName
			key = filepath.Base(header.Name)

			switch header.Typeflag {
			case tar.TypeDir:
				continue
			case tar.TypeReg:
				// save to configmap
				var b bytes.Buffer
				b.ReadFrom(tarReader)
				//fmt.Println("debug contents [%s]", b.String())
				if key != "" {
					fmt.Println("configmap key", key)
					configMapData[key] = b.String()
				}
			case tar.TypeXGlobalHeader, tar.TypeXHeader:
				continue
			default:
				return nil, fmt.Errorf("unknown type: %b in %s", header.Typeflag, header.Name)
			}
		}
	}
	if len(configMapData) > 0 {
		_ = CreateMap(name, ns, configMapData)
	}

	return configMapName, nil
}

// CreateMap - create configmap if needed else update existing map
func CreateMap(name string, ns string, configMapData map[string]string) error {

	var kubeconfig *string
	home := "/root"
	kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset := kubernetes.NewForConfigOrDie(config)

	configMap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: configMapData,
	}

	ctx := context.Background()
	var cm *corev1.ConfigMap
	if cm, err = clientset.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{}); err != nil {

		fmt.Println("new configmap needed", err.Error())

		cm, _ = clientset.CoreV1().ConfigMaps(ns).Create(ctx, &configMap, metav1.CreateOptions{})

		fmt.Printf("create new configmap %+v", cm.Name)

	} else {
		cm, _ = clientset.CoreV1().ConfigMaps(ns).Update(ctx, &configMap, metav1.UpdateOptions{})
		fmt.Println("update configmap", cm.Name)
	}

	_, err = clientset.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		fmt.Println("failed to find configmap ", err.Error())
	}

	fmt.Println("found configmap ", cm.Name)

	return nil

}

func CheckMaps(configName []string, ns string, nameofMap string, k8sClient kubernetes.Interface) bool {
	// add k8sclient here

	ctx := context.Background()
	var cm *corev1.ConfigMap
	isFound := false
	if cm, err := k8sClient.CoreV1().ConfigMaps(ns).Get(ctx, nameofMap, metav1.GetOptions{}); err != nil {

		fmt.Println("new configmap needed", err.Error())

		fmt.Printf("create new configmap %+v", cm.Name)
		return isFound
	}
	// read name of map and make a list of configname
	configName = strings.Split(cm.Data["listofconfigmapname"], ",")
	for _, name := range configName {
		if cm, err := k8sClient.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{}); err != nil {

			fmt.Println("new configmap needed", err.Error())

			fmt.Printf("create new configmap %+v", cm.Name)
			return isFound
		}
	}
	isFound = true
	return isFound
}
