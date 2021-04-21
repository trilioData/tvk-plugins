# Kubernetes Triliovault Preflight Checks

**tvk-preflight** is a kubectl plugin which checks if all the pre-requisites are  
met before installing Triliovault for Kubernetes application in a Kubernetes cluster.

## Pre-requisites:

1. krew - kubectl-plugin manager. Install from [here](https://krew.sigs.k8s.io/docs/user-guide/setup/install/)
2. kubectl - kubernetes command-line tool. Install from [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

**Supported OS and Architectures**:

OS:
- Linux
- darwin

Arch:
- amd64
- x86


## Checks Performed during Preflight

Some checks are performed on system from where the application is installed and some are performed on the K8s cluster.  
The following checks included in preflight:

- Ensure *kubectl* utility is present on system
- Ensure *kubectl* is pointed to k8s cluster (i.e can access the remote target cluster)
- Ensure *helm* utility is present on system and pointed to the cluster
  - If *helmVersion=~v2*, then ensure *tiller* is present on cluster
  - If *helmVersion=~v3*, then *tiller* is not needed on cluster
- Ensure minimum Kubernetes version >= 1.13.x
- Ensure RBAC is enabled in cluster
- Ensure provided storageClass is present in cluster
- Ensure atleast one of the snapshot class is marked as *default* in cluster if volume snapshot api is in alpha state
- Ensure all required features are present
  - Alpha features for k8s version less than 1.14.x and greater than 1.13.x  --> *"CSIBlockVolume" "CSIDriverRegistry" "CSINodeInfo" "VolumeSnapshotDataSource"*
  - Alpha features for k8s version less than 1.17.x and greater than 1.14.x --> *"VolumeSnapshotDataSource"*
  - Alpha features for k8s version less than 1.15.x --> *"CustomResourceWebhookConversion"*
  - No Alpha features required for k8s version >= 1.17.x
- Ensure CSI apis are present in cluster
  - "csidrivers.csi.storage.k8s.io" (Only for k8s 1.13.x)
  - "csinodeinfos.csi.storage.k8s.io" (Only for k8s 1.13.x)
  - "volumesnapshotclasses.snapshot.storage.k8s.io"
  - "volumesnapshotcontents.snapshot.storage.k8s.io"
  - "volumesnapshots.snapshot.storage.k8s.io"
- Ensure DNS resolution works as expected in the cluster
  - Creates a new pod (*dnsutils*) then resolve *kubernetes.default* service from inside the pod
- Ensure Volume Snapshot functionality works as expected for both used and unused PVs
  - Create a source Pod and PVC (*source-pod* and *source-pvc*)
  - Create a Volume snapshot from a used PV (*snapshot-source-pvc*) from the *source-pvc*
  - Create a volume snapshot from unused PV (delete the source pod before snapshoting)
  - Create a restore Pod and PVC (*restored-pod* and *restored-pvc*)
  - Create a resotre Pod and PVC from unused pv snapshot
  - Ensure data in restored pod/pvc
- Cleanup of all the intermediate resources created


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

## Examples

- With `--snapshotclass`:

```shell script
kubectl tvk-preflight --storageclass <storageclass name> --snapshotclass <volumeSnapshotClass name>
```

- Without `--snapshotclass`:

```shell script
kubectl tvk-preflight --storageclass <storageclass name>
```
