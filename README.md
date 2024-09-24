# Hierarchical Namespaces (HNS)
`HNS` is built to allow for easier multi-tenancy in OpenShift clusters, containing a set of CRDs and controllers that allow users to create namespaces without needing cluster-level permission to create namespaces, with each namespace having a quota associated to it.

## Using HNS

### Prerequisites
In order to use `HNS`, you need to have:
1. An operating OpenShift cluster of version 4.x.
2. `cert-manager` installed on the cluster.

## Install with Helm

Helm chart docs are available on `charts/hns` directory.

```bash
$ helm upgrade hns --install --namespace hns-system --create-namespace oci://ghcr.io/dana-team/helm-charts/hns --version <release>
```

### Build
To build the `HNS` controller, login into an image registry, and run:

```
$ make docker-build docker-push IMG=<image_registry>/<image_name>:<image_tag>
```

### Deploy
To build the `HNS` controller, login into an operational OpenShift cluster and run:
```
$ make deploy IMG=<image_registry>/<image_name>:<image_tag>
```

### Test
To test the `HNS` controller, login into an operational OpenShift cluster and run:

```
$ make test-e2e
```

## CRDs
1. `Subnamespace`: Represents a namespace in a hierarchy. Each `Subnamespace` has a `namespace` bound to it which has the same name as the `Subnamespace`. A `Subnamepsace` may also have a quota bound to it. The quota can be either a `ResourceQuota` object or a `ClusterResourceQuota` object (this depends on the depth in the hierarchy of the `Subnamespace`) of the same name as the `Subnamespace`.

2. `UpdateQuota`: A CRD that allows to move resources between `Subnamespaces`.

3. `MigrationHierarchy`: A CRD that allows migrating a `Subnamespace` to a different hierarchy.