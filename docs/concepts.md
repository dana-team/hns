# HNS: Concepts

In this section, the key concepts and the way things operate in the HNS project will be explained. After reading this document you should be able to recognize the core components of the HNS project and understand how and _why_ things work the way they do.

## Why?
### Why use namespaces?

Namespaces are a way to group resources in a Kubernetes cluster. They provide a virtual partitioning of a cluster, allowing different teams or applications to operate in isolation from each other.

By default, Kubernetes clusters have a single namespace called `default`, but you can create additional namespaces as needed. Resources created within a namespace are isolated from resources in other namespaces, allowing for better resource management and security.

Namespaces also provide a way to limit resource consumption within a cluster. For example, you can set resource quotas on a per-namespace basis, which can help prevent one team or application from monopolizing resources within the cluster.

Overall, namespaces are a fundamental concept in Kubernetes that allow for better organization, isolation, and resource management within a cluster.

### Why use HNS?
The `HNS` project comes to solve two problems:

- In large-scale deployments of Kubernetes, you may want to delegate the responsibility of creating new namespaces to end-clients. However, namespace creation is a privileged cluster-level operation, and end-clients should generally not get this kind of privilege.

- It would be convenient to extend the ability of limiting resource consumption beyond the scope of a namespace, and instead control the consumption of several namespaces under a single quota.

`HNS` solves these two problems by allowing you to create a logical _tree_ of namespaces which implements a hierarchy. Each node in this _tree_ is called a `Subnamespace` and end-users can be given permissions to create `Subnamespaces` without needing cluster-level permissions. Each `Subnamespace` has a `ResourceQuota` or `ClusterResourceQuota` associated to it, which allows to extend the scope of the quota past a single namespace.

## Basic Concepts
These concepts are useful for anyone using a cluster with `HNS`.

### Root namespace, secondary-root and trees
For each deployment of `HNS`, a `root namespace` has to be created manually. The `root namespace` is the top of the tree-like hierarchy that `HNS` builds. The `root namespace` should have some labels and annotations set with it:

```
kind: Namespace
apiVersion: v1
metadata:
  name: <root namespace name>
  labels:
    dana-hns.io/aggragtor-<root namespace name>: 'true'
    dana.hns.io/subnamespace: 'true'
  annotations:
    dana.hns.io/role: root
    openshift.io/display-name: <root namespace name>
    dana.hns.io/crq-selector-0: <root namespace name>
    dana.hns.io/depth: '0'
    dana.hns.io/rq-depth: '<rq-depth>'
```

`Secondary roots` are subnamespaces which denote different branches of the hierarchy which are made up of different hardware. For example, nodes that include GPU would be under a different `secondary root` than regular nodes, so that the hierarchy would be: `{root-namespace} -> {gpu, non-gpu}`. `Secondary roots` are denoted by the `dana.hns.io/is-secondary-root: 'True'` annotation. This annotation needs to be added manually to `secondary roots`! Moving resources and migrating subnamespaces between `secondary roots` is not allowed.

If `secondary roots` exist than `<rq-depth>` needs to be set to `2`; alternatively it needs to be set to `1`.

More information regarding the labels and annotations exists [here](#labels-and-annotations).

### CRDs
Leveraging the concept of CRDs, 3 new resources are created as part of the `HNS`:

- `Subnamespace`
- `UpdateQuota`
- `MigrationHierarchy`

#### Namespace-scoped API and Cluster-Scoped API
`Subnamespace` and `UpdateQuota` are namespace-scoped APIs, meaning that their Custom Resources are uniquely identified by a name and a namespace, while `MigrationHierarchy` is a cluster-scoped API, meaning its Custom Resources are uniquely defined by just a name.

### User Capabilities
A regular `HNS` user, who is not a `ClusterAdmin` has the following capabilities on namespaces the user is an `Admin` on (in addition to `Admin` capabilities on the namespace itself to deploy workload etc…):

- `Subnamespace`: `CREATE`, `LIST`, `GET`
- `UpdateQuota`: `CREATE`, `UPDATE`, `LIST`, `GET`, `PATCH`
- `Namespace`: `DELETE`
- `ClusterResourceQuota` (cluster-scoped): `GET`, `LIST`, `WATCH`.

At the moment only a `ClusterAdmin` has any capabilities at all on `MigrationHierarchy` objects.

### Subnamespace
`Subnamespace` (`SNS`) is a Kubernetes CRD that represents a namespace in a hierarchy.

Each `Subnamespace` has a namespace bound to it which has the same name as the `Subnamespace`. A `Subnamepsace` may also have a quota bound to it. The quota can be either a `ResourceQuota` object or a `ClusterResourceQuota` object (this depends on the depth in the hierarchy of the Subnamespace) of the same name as the `Subnamespace`.

The introduction of `Subnamespace` allows for 3 main things that are not possible out of the box:

- Hierarchy of namespaces in a Kubernetes cluster - by design, all namespaces are in a flat, single hierarchy; with `Subnamespaces`, there is a logical hierarchy for namespaces.
- Permissions to create new namespaces - by default, only a user with cluster-level privileges can create new namespaces in the cluster; with Subnamespaces, end-users can create new `Subnamespaces` in their hierarchy, which in turn create new namespaces where workload can be deployed.
- Hierarchical quota limitation - with `Subnamespaces` and `CRQs`/`RQs`, quotas can be limited in a hierarchy, with each level of the hierarchy having a stricter (or equal) limitation than the upper level.

A `Subnamespace` is an object inside the namespace of the parent of the `SNS`. For example, subnamespace `1110` would live inside the namespace `1100`, and `subnamespace` `1100` would be inside NS `1000`.

#### ResourcePool
Each `Subnamespace` has a label that decides whether a `Subnamespace` is a `ResourcePool` or not. When a `Subnamespace` is a `ResourcePool` then it means that (unless this `Subnamespace` is the first `ResourcePool` in its tree branch), then it does not have a `CRQ` or `RQ` bound to it; instead, the `Subnamespace` shares its resources with all the other `Subnamespaces` in the `ResourcePool`.

##### Upper ResourcePool
The first `ResourcePool` in its three branch is called an upper `ResourcePool` (or `upper-rp`) and is denoted by the annotation: `dana.hns.io/is-upper-rp: True`. Other subnamespaces in the `ResourcePool` would have an annotation `dana.hns.io/upper-rp: <upper-rp-name>`.

#### Subnamespace resources
Every `Subnamespace` which is not a `ResourcePool` must have the following quota parameters set in its spec:


- `basic.storageclass.storage.k8s.io/requests.storage:` `<storage quantity>`
- `cpu:` `<cpu quantity>`
- `memory:` `<memory quantity>`
- `pods:` `<pods quantity>`
- `requests.nvidia.com/gpu:` `<gpu quantity>`

#### Examples

An example of a CR of a `Subnamespace` which allows you to create a SNS which is not a `ResourcePool`:

```
apiVersion: dana.hns.io/v1
kind: Subnamespace
metadata:
  name: '1100'
  namespace: '1000'
  labels:
    dana.hns.io/resourcepool: 'false'
spec:
  resourcequota:
    hard:
      basic.storageclass.storage.k8s.io/requests.storage: 50Gi
      cpu: '50'
      memory: 50Gi
      pods: '50'
      requests.nvidia.com/gpu: '0'
```

An example of a CR of a `Subnamespace` which allows you to create a SNS which is the first `ResourcePool` in its tree branch:

```
apiVersion: dana.hns.io/v1
kind: Subnamespace
metadata:
  name: '1100'
  namespace: '1000'
  labels:
    dana.hns.io/resourcepool: 'true'
spec:
  resourcequota:
    hard:
      basic.storageclass.storage.k8s.io/requests.storage: 50Gi
      cpu: '50'
      memory: 50Gi
      pods: '50'
      requests.nvidia.com/gpu: '0'
```

An example of a CR of a `Subnamespace` which allows you to create a SNS which is not the first `ResourcePool` in its tree branch:

```
apiVersion: dana.hns.io/v1
kind: Subnamespace
metadata:
  name: '1110'
  namespace: '1100'
  labels:
    dana.hns.io/resourcepool: 'true'
spec:
  resourcequota: {}
```

#### Hierarchiel Quota Limitation
One of the goals of `HNS` is to extend the idea of `ResourceQuota` past a single namespace and to achieve hierarchical quota limitation in a way that each depth of the tree is managed by the quota object of its ancestors and by the quota object that is bound to it. This way, every `Subnamespace` (and as a result each namespace) has a quota which is lower to equal to that of its direct parent.

This is achieved by using `ClusterResourceQuota` and `ResourceQuota` objects. Every namespace that is created by a `Subnamespace` has `dana.hns.io/crq-selector-<X>` annotations. This annotation corresponds to the selector that exists on the `ClusterResourceQuota` object.

##### ClusterResourceQuota and ResourceQuota
`ClusterResourceQuota` objects are OpenShift objects that allow managing several namespaces under the same quota. However, as per the OpenShift documentation [selecting more than 100 projects](https://docs.openshift.com/container-platform/4.12/applications/quotas/quotas-setting-across-multiple-projects.html#quotas-selection-granularity_setting-quotas-across-multiple-projects) under a single CRQ can have detrimental effects on API server responsiveness in those projects.

Therefore, it is often required to not use `ClusterResourceQuota` created for `Subnamespaces` under which in the hierarchy there can be more than 100 `Subnamespaces`. Therefore, for `Subnamespaces` in high hierarchies, it is better to create a `ResourceQuota` instead of a `ClusterResourceQuota` object.

The depth until which `ResourceQuotas` are created for namespaces is controlled by the `dana.hns.io/rq-depth` annotation on the [root namespace](#root-namespace-secondary-root-and-trees).

Note that `HNS` limits for the number of namespaces that be in a hierarchy, using the `MAX_SNS_IN_HIERARCHY` environment variable in the `manager` container; the default is `100`.

###### Example

```
kind: Namespace
apiVersion: v1
metadata:
  name: brazil
  labels:
    dana.hns.io/role: leaf
    dana.hns.io/aggragator-brazil: 'true'
    dana.hns.io/subnamespace: 'true'
    kubernetes.io/metadata.name: brazil
    dana.hns.io/parent: south-america
    dana.hns.io/resourcepool: 'false'
    dana.hns.io/aggragator-america: 'true'
    dana.hns.io/aggragator-south-america: 'true'
    dana.hns.io/aggragator-world: 'true'
  annotations:
    dana.hns.io/role: leaf
    openshift.io/display-name: fack/world/america/south-america/brazil
    dana.hns.io/crq-selector-0: fack
    dana.hns.io/crq-selector-1: world
    dana.hns.io/crq-selector-2: america
    dana.hns.io/depth: '4'
    dana.hns.io/crq-selector-3: south-america
    dana.hns.io/crq-selector-4: brazil
    dana.hns.io/sns-pointer: brazil
  finalizers:
    - dana.hns.io/delete-sns
```

#### Labels and Annotations
Labels and Annotations are added by `HNS` to both the `subnamespace` and the `namespace` that is bound to the `subnamespace` (they would always have the same name).

##### Subnamespaces

###### Labels
| Name                        | Explanation     |
| ---                         | ---             |
| `dana.hns.io/resourcepool`  | Can be `true` or `false`. Indicates whether the `Subnamespace` is a `ResourcePool` or not |

###### Annotations
| Name                        | Explanation     |
| ---                         | ---             |
| `dana.hns.io/crq-pointer`   | The name of the `ClusterResourceQuota` bound to the `Subnamespace`. In case of a `ResourcePool` this points to the `CRQ` bound to the `upper-rp` SNS |
| `dana.hns.io/is-rq`         | Can be `True` or `False`. Indicates whether the `Subnamespace` has a `ResourceQuota` object bound to it or a `ClusterResourceQuota` object bound to it |
| `dana.hns.io/is-upper-rp`   | Can be `True` or `False`. Indicates whether the `Subnamespace` is the upper `ResourcePool` of the `ResourcePool` it's part of|
| `dana.hns.io/upper-rp`      | The name of the `Subnamespace` which is the upper `ResourcePool` of the ResourcePool it's part of |
| `openshift.io/display-name` | The hierarchial display-name of the Subnamespace: `X/Y/Z` |

##### Namespaces

###### Labels
| Name                           | Explanation     |
| ---                            | ---             |
| `dana.hns.io/aggragator-<X>`   | `X` is the name of a `Subnamespace` this namespace is in the hierarchy of. There are several annotations like this, as many as the `depth` of the namespace |
| `dana.hns.io/parent`           | The name of the parent of the namespace in the hierarchy |
| `dana.hns.io/resourcepool`     | Indicates whether the `Subnamespace` this namespace is bound to is a `ResourcePool` or not |
| `dana.hns.io/role`             | Can be one of `root` (to indicate a root namespace), `none` (to indicate the `Subnamespace` this namespace is bound to has children), and `leaf` (to indicate the `Subnamespace` this namespace is bound to has no children).
| `dana.hns.io/subnamespace`     | If `true` it indicates that this namespace is managed by `HNS` |

## Annotations
| Name                           | Explanation     |
| ---                            | ---             |
| `dana.hns.io/role`             | Can be one of `root` (to indicate a root namespace), `none` (to indicate the `Subnamespace` this namespace is bound to has children), and `leaf` (to indicate the `Subnamespace` this namespace is bound to has no children).
| `openshift.io/display-name`    | The hierarchial display-name of the namespace: `X/Y/Z` |
| `dana.hns.io/crq-selector-<X>` | The selector of a `ClusterResourceQuota` this namespace is managed by. There may be several annotations like this, possibly as many as the `depth` of the namespace  |
| `dana.hns.io/depth`            | The distance of the `namespace` from the root namespace, which is of depth 0 |
| `dana.hns.io/sns-pointer`      | The name of the `Subnamespace` this namespace is bound to |

### UpdateQuota
`Updatequota` is a CRD that allows moving resources between `Subnamespaces`. An `Updatequota` is an object inside the namespace of the SNS from which resources are moved. For example, an `Updatequota` object called `moveCPUFromXtoY` would live inside the namespace `X`.

#### Description Annotation
A description of why resources are moved can be added to the `Updatequota` object as an annotation: `dana.hns.io/description`.

#### Example
An example of a CR of an `Updatequota` which allows you to move resources from `X` to `Y`:

```
apiVersion: dana.hns.io/v1
kind: Updatequota
metadata:
    annotations:
      dana.hns.io/description: 'Moving resources for this great new project'
  namespace: 'X'
  name: 'MoveResourcesFromXToY'
spec:
  destns: 'Y'
  resourcequota:
    hard:
      basic.storageclass.storage.k8s.io/requests.storage: 1Gi
      cpu: '1'
      memory: 1Gi
      pods: '1'
      requests.nvidia.com/gpu: '0'
  sourcens: 'X'
```

### MigrationHierarchy
`Migrationhierarchy` is a CRD that allows moving subnamespaces inside the hierarchy, meaning it allows to set a new `parent` for a `subnamespace`. `Migrationhierarchy` is a cluster-scoped object, meaning that it does not live inside a namespace.

#### Example
An example of a CR of an `Migrationhierarchy` which allows you to move subnamespace `X` to be under `Y`:

```
apiVersion: dana.hns.io/v1
kind: MigrationHierarchy
metadata:
  name: 'XtoY'
spec:
  currentns: 'X'
  tons: 'Y'
```