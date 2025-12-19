package operatorutils

import (
    "context"
    "fmt"
    "os"

    corev1 "k8s.io/api/core/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metatypes "k8s.io/apimachinery/pkg/types"
    "sigs.k8s.io/controller-runtime/pkg/client"

    "gopkg.in/yaml.v3"
)

const DefaultCSMImagesConfigMap = "csm-images"

// ResolveVersionedImage reads <cmName>/versions.yaml, finds the entry by desiredVersion,
// and returns entry[key]. Second return value is true if found.
func ResolveVersionedImage(
    ctx context.Context,
    k8sClient client.Client,
    cmName string,         // pass "" to use DefaultCSMImagesConfigMap
    desiredVersion string, // CR.spec.version
    key string,            // e.g., "dell-csi-replicator", "karavi-authorization-proxy"
) (string, bool, error) {
    if desiredVersion == "" {
        return "", false, nil
    }
    if cmName == "" {
        cmName = DefaultCSMImagesConfigMap
    }

    // Find the CM namespace by listing (preserves your existing behaviour)
    var cmList corev1.ConfigMapList
    if err := k8sClient.List(ctx, &cmList); err != nil {
        return "", false, fmt.Errorf("listing configmaps: %w", err)
    }
    ns := ""
    for _, cm := range cmList.Items {
        if cm.Name == cmName {
            ns = cm.Namespace
            break
        }
    }
    if ns == "" {
        // CM not found: let caller fallback to env.
        return "", false, nil
    }

    // Fetch the CM
    var cm corev1.ConfigMap
    if err := k8sClient.Get(ctx, metatypes.NamespacedName{Name: cmName, Namespace: ns}, &cm); err != nil {
        if apierrors.IsNotFound(err) {
            return "", false, nil
        }
        return "", false, fmt.Errorf("getting %s/%s: %w", ns, cmName, err)
    }

    data, ok := cm.Data["versions.yaml"]
    if !ok {
        return "", false, nil
    }

    // versions.yaml is a list of flat maps (map[string]string)
    var entries []map[string]string
    if err := yaml.Unmarshal([]byte(data), &entries); err != nil {
        return "", false, nil
    }

    // Find the entry with the desired version
    for _, e := range entries {
        if e["version"] == desiredVersion {
            if img := e[key]; img != "" {
                return img, true, nil
            }
            return "", false, nil
        }
    }

    return "", false, nil
}

// ResolveVersionedImageOrEnv returns ConfigMap image for key if available,
// otherwise returns os.Getenv(envFallback). If desiredVersion is empty, returns "".
func ResolveVersionedImageOrEnv(
    ctx context.Context,
    k8sClient client.Client,
    cmName string,         // "" -> DefaultCSMImagesConfigMap
    desiredVersion string, // CR.spec.version
    key string,            // versions.yaml lookup key
    envFallback string,    // RELATED_IMAGE_* env name
) string {
    if desiredVersion == "" {
        return ""
    }
    if img, ok, _ := ResolveVersionedImage(ctx, k8sClient, cmName, desiredVersion, key); ok {
        return img
    }
    return    return os.Getenv(envFallback)
}