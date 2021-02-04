# Kubernetes Triliovault Preflight Checks

**preflight** is a standalone helper script which checks if all the pre-requisites are
met before installing Triliovault for Kubernetes application in a Kubernetes cluster.

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

## Running Preflight checks

- Getting preflight.sh
  - `wget https://raw.githubusercontent.com/triliovault-k8s-issues/triliovault-k8s-issues/master/tools/preflight/preflight.sh`
  - `chmod +x ./preflight.sh`

- Available parametes for script `./preflight.sh --help`
  - `--storageclass` - Name of storage class being used in k8s cluster (Needed)
  - `--snapshotclass` Name of volume snapshot class being used in k8s cluster (Needed)
  - `--kubeconfig` - Kubeconfig path, if not given default is used by kubectl (Optional)

- Running preflight checks
  - `./preflight.sh --storageclass my-hostpath-sc --snapshotclass default-snapclass --kubeconfig /home/usr/kubeconfig`
