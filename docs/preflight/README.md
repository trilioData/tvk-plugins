# TVK Preflight Plugin

**tvk-preflight** is a kubectl plugin which checks if all the pre-requisites are met before installing Triliovault for Kubernetes
application in a Kubernetes cluster.

This plugin automatically generates log file(`preflight-log-<date-time>.log`) for each preflight run which can be used to
get more information around check being performed by this plugin.

## Pre-requisites:

1. `krew`[optional] - kubectl-plugin manager. Install from [here](https://krew.sigs.k8s.io/docs/user-guide/setup/install/)
2. `kubectl` - kubernetes command-line tool. Install from [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
3. `bash`(>=v3.2.x) should be present on system

**Supported OS:**
- Linux
- darwin

## Checks Performed during Preflight

Preflight plugin performs checks on system where this plugin is installed and few checks are performed on the K8s cluster
where current-context of kubeconfig is pointing to.

The following checks are included in preflight:

1. `check-kubectl` - Ensures **kubectl** utility is present on system
2. `check-kubectl-access` - Ensures **kubectl** is pointed to k8s cluster (i.e can access the remote target cluster)
3. `check-helm-version` -
    1. Ensures **helm**[version>=v3.x.x] utility is present on system and pointed to the cluster
    2. Aborts successfully for Openshift cluster
4. `check-kubernetes-version` - Ensures minimum Kubernetes version >= 1.18.x
5. `check-kubernetes-rbac` - Ensures RBAC is enabled in cluster
6. `check-storage-snapshot-class` -
    1. Ensures provided storageClass is present in cluster
        1. Provided storageClass's `provisioner` [JSON Path: `storageclass.provisioner`] should match with provided volumeSnapshotClass's `driver`[JSON Path: `volumesnapshotclass.driver`]
        2. If volumeSnapshotClass is not provided then, volumeSnapshotClass which satisfies condition `[i]` will be selected.
        If there's are multiple volumeSnapshotClasses satisfying condition `[i]`, default volumeSnapshotClass[which has annotation `snapshot.storage.kubernetes.io/is-default-class: "true"` set]
        will be used for further pre-flight checks.
        3. Pre-flight check fails if no volumeSnapshotClass is found[after considering all above mentioned conditions].
    2. Ensures at least one volumeSnapshotClass is marked as *default* in cluster if user has not provided volumeSnapshotClass as input.
7. `check-csi` -
    1. Ensure following CSI apis are present in cluster -
        - "volumesnapshotclasses.snapshot.storage.k8s.io"
        - "volumesnapshotcontents.snapshot.storage.k8s.io"
        - "volumesnapshots.snapshot.storage.k8s.io"
9. `check-dns-resolution` -
    1. Ensure DNS resolution works as expected in the cluster
        - Creates a new pod (**dnsutils-${RANDOM_STRING}**) then resolves **kubernetes.default** service from inside the pod
10. `check_volume_snapshot` - 
    1. Ensure Volume Snapshot functionality works as expected for both mounted and unmounted PVCs
        1. Creates a Pod and PVC (**source-pod-${RANDOM_STRING}** and **source-pvc-${RANDOM_STRING}**).
        2. Creates Volume snapshot (**snapshot-source-pvc-${RANDOM_STRING}**) from the mounted PVC(**source-pvc-${RANDOM_STRING}**).
        3. Creates volume snapshot of unmounted PVC(**source-pvc-${RANDOM_STRING}** [deletes the source pod before snapshotting].
        4. Restores PVC(**restored-pvc-${RANDOM_STRING}**) from volume snapshot of mounted PVC and creates a Pod(**restored-pod-${RANDOM_STRING}**) and attaches to restored PVC.
        5. Restores PVC(**unmounted-restored-pvc-${RANDOM_STRING}**) from volume snapshot from unmounted PVC and creates a Pod(**unmounted-restored-pod-${RANDOM_STRING}**) and attaches to restored PVC.
        6. Ensure data in restored PVCs is correct[checks for a file[/demo/data/sample-file.txt] which was present at the time of snapshotting].
    2. If `check-storage-snapshot-class` fails then, `check_volume_snapshot` check is skipped.

After all above checks are performed, cleanup of all the intermediate resources created during preflight checks' execution is done.


## Installation, Upgrade, Removal of Plugins :

#### 1. With `krew`:

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


#### 2. Without `krew`:

1. List of available releases: https://github.com/trilioData/tvk-plugins/releases
2. Choose a version of preflight plugin to install and check if release assets have preflight plugin's package[preflight.tar.gz]
3. Set env variable `version=v1.x.x` [update with your desired version].

##### Linux/macOS

- Bash or ZSH shells
```bash
(
  set -ex; cd "$(mktemp -d)" &&
  curl -fsSLO "https://github.com/trilioData/tvk-plugins/releases/download/"${version}"/preflight.tar.gz" &&
  tar zxvf preflight.tar.gz && sudo mv preflight/preflight /usr/local/bin/kubectl-tvk_preflight
)
```
Verify installation with `kubectl tvk-preflight --help`

##### Windows
NOT SUPPORTED


## Usage:

    kubectl tvk-preflight [flags]
	
- Flags:

| Parameter                 | Default       | Description   |    
| :------------------------ |:-------------:| :-------------|  
| --storageclass          |             |Name of storage class being used in k8s cluster (Needed)
| --snapshotclass          |            |Name of volume snapshot class being used in k8s cluster (Optional)
| --kubeconfig            |   ~/.kube/config             |Kubeconfig path, if not given default is used by kubectl (Optional)

## Examples

- With `--snapshotclass`:

```shell script
kubectl tvk-preflight --storageclass <storageclass name> --snapshotclass <volumeSnapshotClass name>
```

- Without `--snapshotclass`:

```shell script
kubectl tvk-preflight --storageclass <storageclass name>
```
