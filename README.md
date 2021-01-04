# Metabroker

Metabroker is an implementation of a service broker for Kubernetes. It provides
a framework for creating service offerings and plans for provisioning Helm
charts as service instances.

## Trying Metabroker locally

1. Start Minikube:

```shell
minikube start --cpus 4 --embed-certs
```

2. Install the CRDs:

```shell
(cd "$(git rev-parse --show-toplevel)/apis/"; make install-crd)
```

3. Run the Metabroker operator:

```shell
(cd "$(git rev-parse --show-toplevel)/operator/"; make run)
```

4. Install the postgres package example:

```
helm upgrade --install postgres-metabroker "$(git rev-parse --show-toplevel)/examples/packages/postgres-metabroker/chart/postgres-metabroker/"
```

5. Build the CLI:

```shell
(cd "$(git rev-parse --show-toplevel)/cli/"; make)
```

6. Provision an instance:

```shell
cd "$(git rev-parse --show-toplevel)/cli/"
./.bin/metabrokerctl provision mypg postgres-metabroker-small
```

7. Bind the instance:

```shell
cd "$(git rev-parse --show-toplevel)/cli/"
./.bin/metabrokerctl bind mypg mypg-creds
```

8. Check other `metabrokerctl` commands and try them out:

```shell
cd "$(git rev-parse --show-toplevel)/cli/"
./.bin/metabrokerctl --help
```
