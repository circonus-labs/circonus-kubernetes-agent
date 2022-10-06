package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func GetClient(clusterConfig *config.Cluster) (*kubernetes.Clientset, error) {
	var cfg *rest.Config
	if c, err := rest.InClusterConfig(); err != nil {
		if !errors.Is(err, rest.ErrNotInCluster) {
			return nil, fmt.Errorf("unable to configure k8s api client: %w", err)
		}
		// not in cluster, use supplied customer config for cluster
		cfg = &rest.Config{}
		if clusterConfig.BearerToken != "" {
			cfg.BearerToken = clusterConfig.BearerToken
		}
		if clusterConfig.URL != "" {
			cfg.Host = clusterConfig.URL
		}
		if clusterConfig.CAFile != "" {
			cfg.TLSClientConfig = rest.TLSClientConfig{CAFile: clusterConfig.CAFile}
		}
	} else {
		cfg = c // use in-cluster config
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("initializing k8s api Clientset: %w", err)
	}

	return clientset, nil
}

// GetVersion gets the cluster version
func GetVersion(ctx context.Context, clusterConfig *config.Cluster) (string, error) {
	clientset, err := GetClient(clusterConfig)
	if err != nil {
		return "", err
	}
	req := clientset.CoreV1().RESTClient().Get().RequestURI("/version")
	res := req.Do(ctx)

	data, err := res.Raw()
	if err != nil {
		return "", err
	}

	var ver apimachineryversion.Info
	if err := json.Unmarshal(data, &ver); err != nil {
		return "", err
	}

	return ver.GitVersion, nil
}

// GetVersionPlatform gets the cluster version + " " + platform
func GetVersionPlatform(ctx context.Context, clusterConfig *config.Cluster) (string, error) {
	clientset, err := GetClient(clusterConfig)
	if err != nil {
		return "", err
	}
	req := clientset.CoreV1().RESTClient().Get().RequestURI("/version")
	res := req.Do(ctx)

	data, err := res.Raw()
	if err != nil {
		return "", err
	}

	var ver apimachineryversion.Info
	if err := json.Unmarshal(data, &ver); err != nil {
		return "", err
	}

	return ver.GitVersion + " " + ver.Platform, nil
}
