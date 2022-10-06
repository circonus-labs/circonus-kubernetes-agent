# circonus-kubernetes-agent prow CI jobs

> Here be dragons

## Dependencies

The following are required, outside of (but not excluding) utilities
normally available on macos

- yq ()
- terraform ()
- helm ()
- helmfile ()
- git ()
- bash ()
- ssh ()
- gcloud ()

## Manual deploy

A deployment process as easy as `seq 1 3`

0. Ensure your system has the required dependencies
1. `cd` to this directory.
2. run `make`
3. ???

## Automatic Deploy

> TODO

## TODO

- Docker container
- S3/GCS bucket for tfstate
- Automatic Deploy

