module github.com/circonus-labs/circonus-kubernetes-agent

go 1.13

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20210608160410-67692ebc98de
	github.com/alecthomas/units v0.0.0-20210208195552-ff826a37aa15
	github.com/circonus-labs/circonus-gometrics/v3 v3.4.6
	github.com/circonus-labs/go-apiclient v0.7.15
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.3.0
	github.com/googleapis/gnostic v0.4.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.0
	github.com/hashicorp/go-version v1.3.0
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/klauspost/compress v1.11.4
	github.com/pelletier/go-toml v1.9.4
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.29.0
	github.com/rs/zerolog v1.25.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.9.0
	go.uber.org/automaxprocs v1.4.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210823070655-63515b42dcdf
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	k8s.io/utils v0.0.0-20200124190032-861946025e34 // indirect
)
