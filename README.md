# Metabroker

Metabroker is an implementation of a service broker for Kubernetes. It provides
a framework for creating service offerings and plans for provisioning Helm
charts as service instances.

## Trying Metabroker locally

All commands are assumed to run from the project root.

1. Start Minikube:

```shell
minikube start --cpus 4 --embed-certs
```

2. Install the CRDs:

```shell
make --directory apis install-crd
```

3. Run the Metabroker operator:

```shell
make --directory operator run
```

4. Install the postgres package example:

```
helm upgrade --install postgres-metabroker \
    "examples/packages/postgres-metabroker/chart/postgres-metabroker/"
```

5. Build the CLI:

```shell
make --directory cli
```

6. Provision an instance:

```shell
cli/.bin/metabrokerctl provision mypg postgres-metabroker-small
```

7. Bind the instance:

```shell
cli/.bin/metabrokerctl bind mypg mypg-creds
```

8. Check other `metabrokerctl` commands and try them out:

```shell
cli/.bin/metabrokerctl --help
```
