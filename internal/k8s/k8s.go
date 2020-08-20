package k8s

import (
	"encoding/json"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/pkg/errors"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func GetClient(clusterConfig *config.Cluster) (*kubernetes.Clientset, error) {
	var cfg *rest.Config
	if c, err := rest.InClusterConfig(); err != nil {
		if err != rest.ErrNotInCluster {
			return nil, errors.Wrap(err, "unable to configure k8s api client")
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
		return nil, errors.Wrap(err, "initializing k8s api Clientset")
	}

	return clientset, nil
}

func GetVersion(clusterConfig *config.Cluster) (string, error) {
	clientset, err := GetClient(clusterConfig)
	if err != nil {
		return "", err
	}
	req := clientset.CoreV1().RESTClient().Get().RequestURI("/version")
	res := req.Do()

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
