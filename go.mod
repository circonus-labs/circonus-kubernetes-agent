module github.com/circonus-labs/circonus-kubernetes-agent

go 1.13

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20190819182555-854d396b647c
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d
	github.com/circonus-labs/go-apiclient v0.7.0
	github.com/google/uuid v1.1.1
	github.com/hashicorp/go-retryablehttp v0.6.4 // indirect
	github.com/pelletier/go-toml v1.6.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_model v0.0.0-20191202183732-d1d2010b5bee
	github.com/prometheus/common v0.7.0
	github.com/rs/zerolog v1.17.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.5.0
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20191204072324-ce4227a45e2e
	gopkg.in/yaml.v2 v2.2.7
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.17.0
)
