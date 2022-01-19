# TVK Preflight Plugin

**tvk-preflight** is a kubectl plugin which checks if all the pre-requisites are met before installing Triliovault for Kubernetes
application in a Kubernetes cluster.

This plugin automatically generates log file for each preflight run(`preflight-<date>T<time>.log`) 
and cleanup(`preflight_cleanup-<date>T<time>.log`) which can be used to get more information around operations being performed by this plugin.

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

Whenever a preflight check is performed, a 6-length smallcase alphabet
`UID` is generated for that particular preflight check. This `UID` is the value of the label `preflight-run` which is set on every
resource created during the preflight check. Also the `UID` is the suffix of name of every resource created during preflight check.
This `UID` is particularly useful to perform cleanup of resources created during a particular preflight check.

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
        - Creates a new pod (**dnsutils-${UID}**) then resolves **kubernetes.default** service from inside the pod
10. `check_volume_snapshot` - 
    1. Ensure Volume Snapshot functionality works as expected for both mounted and unmounted PVCs
        1. Creates a Pod and PVC (**source-pod-${UID}** and **source-pvc-${UID}**).
        2. Creates Volume snapshot (**snapshot-source-pvc-${UID}**) from the mounted PVC(**source-pvc-${UID}**).
        3. Creates volume snapshot of unmounted PVC(**source-pvc-${UID}** [deletes the source pod before snapshotting].
        4. Restores PVC(**restored-pvc-${UID}**) from volume snapshot of mounted PVC and creates a Pod(**restored-pod-${UID}**) and attaches to restored PVC.
        5. Restores PVC(**unmounted-restored-pvc-${UID}**) from volume snapshot from unmounted PVC and creates a Pod(**unmounted-restored-pod-${UID}**) and attaches to restored PVC.
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
3. Set env variable `version=v1.x.x` [update with your desired version]. If `version` is not exported, `latest` tagged version
   will be considered.

##### Linux/macOS

- Bash or ZSH shells
```bash
(
  set -ex; cd "$(mktemp -d)" &&
  if [[ -z ${version} ]]; then version=$(curl -s https://api.github.com/repos/trilioData/tvk-plugins/releases/latest | grep -oP '"tag_name": "\K(.*)(?=")'); fi &&
  echo "Installing version=${version}" &&
  curl -fsSLO "https://github.com/trilioData/tvk-plugins/releases/download/"${version}"/preflight.tar.gz" &&
  tar zxvf preflight.tar.gz && sudo mv preflight/preflight /usr/local/bin/kubectl-tvk_preflight
)
```
Verify installation with `kubectl tvk-preflight --help`

##### Windows
NOT SUPPORTED

**Note for Dark Site Installation**

- For using the `local-registry` flag, it is mandatory to have `busybox:latest` and `dnsutils:1.3` images (with the same tags) to be there in the private registry. 

    > Steps for pushing images to local registry
    - Pull the images (dnsutils:1.3 & busybox) to local machine.
    - use the following command to push it to the local registry
    - `docker push <local registry/image>` 
    - Example: `docker push localhost:5000/busybox`
## Usage:

    kubectl tvk-preflight [sub-command] [flags]

The preflight binary has three common flags to both the subcommands.

#### Common Flags
| Parameter     | Shorthand  | Default       | Description   |    
| :-------------| :----------|:-------------:| :-------------|  
| --namespace   | -n       | default        | Namespace of the cluster in which resources will be created, preflight checks will be performed or resources will cleaned. Default is 'default' namespace of the cluster (Optional)
| --kubeconfig  | -k       | ~/.kube/config | kubeconfig file path (Optional)
| --log-level   | -l       | INFO           | Logging level for the preflight check and cleanup. Logging levels are FATAL, ERROR, WARN, INFO, DEBUG (Optional)

#### Examples

- With `--namespace`:

```shell script
kubectl tvk-preflight [sub-command] [sub-command flags] --namespace <namespace of the cluster>
```

By using shorthand notation:
```shell script
kubectl tvk-preflight [sub-command] [sub-command flags] -n <namespace of the cluster>
```

- With `--kubeconfig`:

```shell script
kubectl tvk-preflight [sub-command] [sub-command flags] --kubeconfig <kubeconfig file path>
```

By using shorthand notation:
```shell script
kubectl tvk-preflight [sub-command] [sub-command flags] -k <kubeconfig file path>
```

- With `--log-level`:

```shell script
kubectl tvk-preflight [sub command] [sub-command flags] --log-level <logging level>
```

By using shorthand notation:

```shell script
kubectl tvk-preflight [sub command] [sub-command flags] -l <logging level>
```

There are two subcommands to the preflight binary:
- **run**: To perform preflight checks
- **cleanup**: To clean the resources generated during failed preflight checks

### 1. run
**run** sub-command performs the actual preflight checks on the system and on the kubernetes cluster where the system's 
kubeconfig is pointing to in the given namespace.
	
#### Flags:

| Parameter                 | Default       | Description   |    
| :------------------------ |:-------------:| :-------------|  
| --storage-class         |             | Name of storage class being used in k8s cluster (Needed)
| --volume-snapshot-class |             | Name of volume snapshot class being used in k8s cluster (Optional)
| --local-registry        |             | Name of the local registry from where the images will be pulled (Optional)
| --image-pull-secret     |             | Name of the secret for authentication while pulling the images from the local registry (Optional)
| --service-account       |             | Name of the service account (Optional)
| --cleanup-on-failure    |   false     | Deletes/Cleans all resources created for that particular preflight check from the cluster even if the preflight check fails. For successful execution of preflight checks, the resources are deleted from cluster by default


#### Examples

Storage-class is a required flag for **run** subcommand.

- With `--volume-snapshot-class`: Performs preflight checks on the cluster with the given volumeSnapshotClass in the given namespace.

```shell script
kubectl tvk-preflight --storage-class <storageclass name> --volume-snapshot-class <volumeSnapshotClass name>
```

- With `--local-registry` | `--service-account`: Performs preflight checks on the cluster with the given local-registry and service-account in the given namespace.

```shell script
kubectl tvk-preflight run --storage-class <storageclass name> --local-registry <local registry file path/name> --service-account <service account name>
```

- With `--image-pull-secret`: To use image-pull-secret, local-registry flag value must be provided. Vice-versa is not true.
```shell script
kubectl tvk-preflight run --storage-class <storageclass name> --local-registry <local registry file path/name> --image-pull-secret <image pull secret name>
```

- With `--cleanup-on-failure`: If preflight checks fail, the resources generated during preflight will be cleaned.

```shell script
kubectl tvk-preflight run --cleanup-on-failure
```

### 2. cleanup
- **cleanup** subcommand cleans/deletes the resources created during failed preflight checks and not cleaned-up on failure.
- The **cleanup** command will clean all the resources generated due to preflight checks in the given namespace.
- User can clean resources of a particular preflight check by specifying the `uid` of the preflight check.

#### Flags:
| Parameter                 | Default       | Description   |    
| :------------------------ |:-------------:| :-------------|  
| --uid                   |             | A 6-length character string generated during preflight check

#### Examples:

- Without `uid`: Cleans all the preflight resources present on the cluster in the given namespace.

```shell script
kubectl tvk-preflight cleanup
```

- With `uid`: Cleans the resources of particular preflight check on the cluster in the given namespace
```shell script
kubectl tvk-preflight cleanup --uid <generated UID of the preflight check>
```
