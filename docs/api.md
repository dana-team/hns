# API Reference

## Packages
- [dana.hns.io/v1](#danahnsiov1)

## dana.hns.io/v1

Package v1 contains API Schema definitions for the dana v1 API group

### Resource Types
- [MigrationHierarchy](#migrationhierarchy)
- [MigrationHierarchyList](#migrationhierarchylist)
- [Subnamespace](#subnamespace)
- [SubnamespaceList](#subnamespacelist)
- [Updatequota](#updatequota)
- [UpdatequotaList](#updatequotalist)

#### MigrationHierarchy
MigrationHierarchy is the Schema for the migrationhierarchies API

_Appears in:_
- [MigrationHierarchyList](#migrationhierarchylist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `dana.hns.io/v1`
| `kind` _string_ | `MigrationHierarchy`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[MigrationHierarchySpec](#migrationhierarchyspec)_ |  |

#### MigrationHierarchyList
MigrationHierarchyList contains a list of MigrationHierarchy

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `dana.hns.io/v1`
| `kind` _string_ | `MigrationHierarchyList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[MigrationHierarchy](#migrationhierarchy) array_ |  |

#### MigrationHierarchySpec
MigrationHierarchySpec defines the desired state of MigrationHierarchy

_Appears in:_
- [MigrationHierarchy](#migrationhierarchy)

| Field | Description |
| --- | --- |
| `currentns` _string_ | CurrentNamespace is name of the Subnamespace that is being migrated |
| `tons` _string_ | ToNamespace is the name of the Subnamespace that represents the new parent of the Subnamespace that needs to be migrated |

#### Namespaces
_Appears in:_
- [SubnamespaceStatus](#subnamespacestatus)

| Field | Description |
| --- | --- |
| `namespace` _string_ | Namespace is the name of a Subnamespace |
| `resourcequota` _[ResourceQuotaSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcequotaspec-v1-core)_ | ResourceQuotaSpec represents the quota allocated to the Subnamespace |

#### Phase
_Underlying type:_ `string`

_Appears in:_
- [MigrationHierarchyStatus](#migrationhierarchystatus)
- [SubnamespaceStatus](#subnamespacestatus)
- [UpdatequotaStatus](#updatequotastatus)

#### Subnamespace
Subnamespace is the Schema for the subnamespaces API

_Appears in:_
- [SubnamespaceList](#subnamespacelist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `dana.hns.io/v1`
| `kind` _string_ | `Subnamespace`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[SubnamespaceSpec](#subnamespacespec)_ |  |

#### SubnamespaceList
SubnamespaceList contains a list of Subnamespace

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `dana.hns.io/v1`
| `kind` _string_ | `SubnamespaceList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[Subnamespace](#subnamespace) array_ |  |

#### SubnamespaceSpec
SubnamespaceSpec defines the desired state of Subnamespace

_Appears in:_
- [Subnamespace](#subnamespace)

| Field | Description |
| --- | --- |
| `resourcequota` _[ResourceQuotaSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcequotaspec-v1-core)_ | ResourceQuotaSpec represents the limitations that are associated with the Subnamespace. This quota represents both the resources that can be allocated to children Subnamespaces and the overall maximum quota consumption of the current Subnamespace and its children. |
| `namespaceRef` _[namespaceRef](#namespaceref)_ | The name of the namespace that this Subnamespace is bound to |

#### Total
_Appears in:_
- [SubnamespaceStatus](#subnamespacestatus)

| Field | Description |
| --- | --- |
| `allocated` _object (keys:[ResourceName](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcename-v1-core), values:Quantity)_ | Allocated is a set of (resource name, quantity) pairs representing the total resources that are allocated to the children Subnamespaces of a Subnamespace. |
| `free` _object (keys:[ResourceName](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcename-v1-core), values:Quantity)_ | Free is a set of (resource name, quantity) pairs representing the total free/available/allocatable resources that can still be allocated to the children Subnamespaces of a Subnamespace. |

#### Updatequota
Updatequota is the Schema for the updatequota API

_Appears in:_
- [UpdatequotaList](#updatequotalist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `dana.hns.io/v1`
| `kind` _string_ | `Updatequota`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[UpdatequotaSpec](#updatequotaspec)_ |  |

#### UpdatequotaList
UpdatequotaList contains a list of Updatequota

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `dana.hns.io/v1`
| `kind` _string_ | `UpdatequotaList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[Updatequota](#updatequota) array_ |  |

#### UpdatequotaSpec
UpdatequotaSpec defines the desired state of Updatequota

_Appears in:_
- [Updatequota](#updatequota)

| Field | Description |
| --- | --- |
| `resourcequota` _[ResourceQuotaSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcequotaspec-v1-core)_ | ResourceQuotaSpec represents resources that need to be transferred from one Subnamespace to another |
| `destns` _string_ | DestNamespace is the name of the Subnamespace to which resources need to be transferred |
| `sourcens` _string_ | SourceNamespace is name of the Subnamespace from which resources need to be transferred |
