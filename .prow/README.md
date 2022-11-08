# circonus-kubernetes-agent prow CI jobs

> Here be dragons

## Dependencies

The following are required, outside of (but not excluding) utilities
normally available on macos

- gcloud ()
- terraform ()
- yq ()
- helm ()
- helmfile ()
- git ()
- bash ()
- ssh ()

## Manual

### Manual deploy

A deployment process as easy as `seq 1 3`

0. Ensure the host system has the required dependencies installed
1. `cd` to this directory.
2. run `make` **note: the tf can take up to 20 minutes to deploy**
3. ???

### Manual teardown

1. run `make clean`
2. ???

### Troubleshooting manual deploy/teardown

Just run the step again.
They should all be idempotent.

## Automatic

### Automatic Deploy

> TODO

### TODO

- Artifact registry / push k8s agent docker container
- package all dependencies in a docker image (alpine based?)
- S3/GCS bucket for tfstate
- Automatic prow-triggered Deploy

