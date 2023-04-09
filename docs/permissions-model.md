# HNS: Permissions Model
In this section, it is explained how the permissions model works in the HNS solution. This section assumes familiarity with HNS concepts.

## Subnamespace Admin
- The Admin of a Subnamespace is hierarchical.
- If a user gets Admin permissions on an OpenShift project then it also gets these Admin permissions on the Subnamespace and on the namespace itself (so the user can deploy workloads).
- The following diagram explains it:

![sns-admin](https://github.com/dana-team/hns/blob/hns-docs/docs/images/sns-admin.svg?raw=true)

## UpdateQuota Permissions

### Moving resources between secondary roots
- It is forbidden to move resources between `Subnamespaces` from different `secondary roots`. This is because `secondary roots` have different hardware behind them and the logical resources from different secondary roots do not translate to the same physical resources.
- For instance in this diagram (`UPDATE QUOTA from FG1 to DE2`)
    - Even if `User D` has `admin permissions` on both `Subnamespace` `FG1` and `Subnamespace` `DE2`, it is impossible to move resources between them because they are from different `secondary roots`.
    - Even `User A`, who is the `admin` on the `root namespace`, can’t move resources between `subnamespaces` from different `secondary roots`.

![updatequota-1](https://github.com/dana-team/hns/blob/hns-docs/docs/images/updatequota-1.svg?raw=true)

### Permissions on the ancestor
- If a user has permissions on the joint ancestor of two `Subnamespaces` then `UpdateQuota` is allowed.
- For instance in this diagram (`UPDATE QUOTA from FG1 to DE2`)
    - The joint ancestor of `FG1` and `DE2` is ABC, so only `User A`, which has permissions on `ABC` can perform the `UpdateQuota`.
    - Any other user would get an error indicating that it does not have sufficient permissions.

![updatequota-2](https://github.com/dana-team/hns/blob/hns-docs/docs/images/updatequota-2.svg?raw=true)

### Permissions on source and dest
If a user has permissions on both the source and destination `Subnamespaces` then `UpdateQuota` is allowed.
- For instance in this diagram (`UPDATE QUOTA from FG1 to DE2`)
    - `User D` has permissions on `FG1` and on `DE2` so it can perform `UpdateQuota` without needing to be admin on the joint ancestor `ABC`.

![updatequota-3](https://github.com/dana-team/hns/blob/hns-docs/docs/images/updatequota-3.svg?raw=true)

### Giving back resources
If a user has permissions on a `Subnamespace` and it tries to move resources up its branch (because it doesn’t need the resources anymore), then it’s allowed as long as it moves resources upwards and to a destination `Subnamespace` in its direct branch.
- For instance in this diagram (`UPDATE QUOTA from FG1 to ABC`)
    - `User F` has permissions on `FG1` and it tries to move resources up the branch so it’s allowed.

![updatequota-4](https://github.com/dana-team/hns/blob/hns-docs/docs/images/updatequota-4.svg?raw=true)

## MigrationHierarchy Permissions

### Migrating between secondary roots
- Same case as `UpdateQuota`, it’s forbidden to migrate between different `secondary roots`.

### Permissions on source and dest
- If a user has permissions on both the source and destination `Subnamespaces` then `MigrationHierarchy` is allowed.
- At the moment `MigrationHierarchy` is only for `ClusterAdmins`, so they always have this permission, but this may change in the future.