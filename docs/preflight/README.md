# TVK Preflight Plugin

**tvk-preflight** is a kubectl plugin which checks if all the pre-requisites are met before installing Triliovault for Kubernetes
application in a Kubernetes cluster.

This plugin automatically generates log file(`preflight-log-<date-time>.log`) for each preflight run which can be used to
get more information around check being performed by this plugin.

## Pre-requisites:

1. krew - kubectl-plugin manager. Install from [here](https://krew.sigs.k8s.io/docs/user-guide/setup/install/)
2. kubectl - kubernetes command-line tool. Install from [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
3. bash(>=v3.2.x) should be present on system

**Supported OS:**
- Linux
- darwin

## Checks Performed during Preflight

Preflight plugin performs checks on system where this plugin is installed and few checks are performed on the K8s cluster
where current-context of kubeconfig is pointing to.

The following checks are included in preflight:

- Ensures *kubectl* utility is present on system
- Ensures *kubectl* is pointed to k8s cluster (i.e can access the remote target cluster)
- Ensures *helm*  utility is present on system and pointed to the cluster
  - If *helmVersion=~v3*, then *tiller* is not needed on cluster
- Ensures minimum Kubernetes version >= 1.18.x
- Ensures RBAC is enabled in cluster
- Ensures provided storageClass is present in cluster
  1. Provided storageClass's `provisioner` [JSON Path: `storageclass.provisioner`] should match with provided volumeSnapshotClass's `driver`[JSON Path: `volumesnapshotclass.driver`]
  2. If volumeSnapshotClass is not provided then, volumeSnapshotClass which satisfies condition `[i]` will be selected.
  If there's are multiple volumeSnapshotClasses satisfying condition `[i]`, default volumeSnapshotClass[which has annotation `snapshot.storage.kubernetes.io/is-default-class: "true"` set]
  will be used for further pre-flight checks.
  3. Pre-flight check fails if no volumeSnapshotClass is found[after considering all above mentioned conditions].
- Ensures at least one volumeSnapshotClass is marked as *default* in cluster if user has not provided volumeSnapshotClass as input.
- Ensures all required features are present
  - No Alpha features required for k8s version >= 1.17.x
- Ensure CSI apis are present in cluster
  - "volumesnapshotclasses.snapshot.storage.k8s.io"
  - "volumesnapshotcontents.snapshot.storage.k8s.io"
  - "volumesnapshots.snapshot.storage.k8s.io"
- Ensure DNS resolution works as expected in the cluster
  - Creates a new pod (*dnsutils-${RANDOM_STRING}*) then resolves *kubernetes.default* service from inside the pod
- Ensure Volume Snapshot functionality works as expected for both used and unused PVs
  - Creates a source Pod and PVC (*source-pod-${RANDOM_STRING}* and *source-pvc-${RANDOM_STRING}*)
  - Creates a Volume snapshot from a used PV (*snapshot-source-pvc-${RANDOM_STRING}*) from the *source-pvc-${RANDOM_STRING}*
  - Creates a volume snapshot from unused PV (delete the source pod before snapshoting)
  - Creates a restore Pod and PVC (*restored-pod-${RANDOM_STRING}* and *restored-pvc-${RANDOM_STRING}*)
  - Creates a restore Pod and PVC from unused pv snapshot
  - Ensure data in restored pod/pvc is correct
- Cleanup of all the intermediate resources created during preflight checks' execution.


## Installation, Upgrade, Removal of Plugins :

- Add TVK custom plugin index of krew:

  ```
  kubectl krew index add tvk-plugins https://github.com/trilioData/tvk-plugins.git
  ```

- Installation:

  ```
  kubectl krew install tvk-plugins/tvk-preflight
  ```  

- Upgrade:

  ```
  kubectl krew upgrade tvk-preflight
  ```  

- Removal:

  ```
  kubectl krew uninstall tvk-preflight
  ```  

## Usage:

    kubectl tvk-preflight [flags]
	
- Flags:

| Parameter                 | Default       | Description   |    
| :------------------------ |:-------------:| :-------------|  
| --storageclass          |             |Name of storage class being used in k8s cluster (Needed)
| --snapshotclass          |            |Name of volume snapshot class being used in k8s cluster (Optional)
| --kubeconfig            |   ~/.kube/config             |Kubeconfig path, if not given default is used by kubectl (Optional)
| --local-registry        |             | Name of the local registry from where the images will be pulled (Optional)
| --image-pull-secret     |             | Name of the secret for authentication while pulling the images from the local registry (Optional)


## Examples

- With `--snapshotclass`:

```shell script
kubectl tvk-preflight --storageclass <storageclass name> --snapshotclass <volumeSnapshotClass name>
```

- Without `--snapshotclass`:

```shell script
kubectl tvk-preflight --storageclass <storageclass name>
```
