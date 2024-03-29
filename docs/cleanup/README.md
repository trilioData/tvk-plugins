# TVK Cleanup Plugin

**tvk-cleanup** is a kubectl plugin which cleans up Triliovaultfor Kubernetes 
application, Helm charts, Custom reources and CRDs in a Kubernetes cluster.

## Pre-requisites:

1. krew - kubectl-plugin manager. Install from [here](https://krew.sigs.k8s.io/docs/user-guide/setup/install/)
2. kubectl - kubernetes command-line tool. Install from [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

For openshift environments, if `kubectl` is not installed and `oc` binary is installed on host machine, then `oc` binary
can be used to perform `kubectl` operation by creating symlink with -
```bash
sudo ln -s /usr/local/bin/oc /usr/local/bin/kubectl
```
Note: 
- `oc` binary path can found by executing `which oc`
- To delete/remove symbolic links use either `unlink` or `rm` command -
```bash
unlink /usr/local/bin/kubectl
```

**Supported OS:**
- Linux
- darwin

## TVK Cleanup

This plugin cleans up all TVK Custom Resources, CRDs, and TVK application itself from all the namespace.
It cleans up TVK installed as operator (on OCP platform) and as helm chart on upstream k8s (Rancher) cluster.

Please note the following:
- Ensure *kubectl* utility is present on system
- Ensure *kubectl* is pointed to k8s cluster (i.e can access the remote target cluster)
- Ensure *helm* utility is present on system and pointed to the cluster
  - *helmVersion=~v3* is needed on the cluster
- Ensure minimum Kubernetes version >= 1.18.x
- Cleans up all the Triliovault Custom Resources, Triliovault Manager application and CRDs from all the namespaces
- User can select to delete any or all of 
  1. Triliovault Application (Operator or Helm chart)
  2. Triliovault CRDs
  3. Triliovault Customer Resources as listed here - 
     Restore Backup Backupplan Hook Target Policy License


## Installation, Upgrade, Removal of Plugins :

#### 1. With `krew`:

- Add TVK custom plugin index of krew:

  ```
  kubectl krew index add tvk-plugins https://github.com/trilioData/tvk-plugins.git
  ```

- Installation:

  ```
  kubectl krew install tvk-plugins/tvk-cleanup
  ```  

- Upgrade:

  ```
  kubectl krew upgrade tvk-cleanup
  ```  

- Removal:

  ```
  kubectl krew uninstall tvk-cleanup
  ```  

#### 2. Without `krew`:

1. List of available releases: https://github.com/trilioData/tvk-plugins/releases
2. Choose a version of preflight plugin to install and check if release assets have preflight plugin's package[cleanup.tar.gz]
3. Set env variable `version=v1.x.x` [update with your desired version]. If `version` is not exported, `latest` tagged version
   will be considered.

##### Linux/macOS

- Bash or ZSH shells
```bash
(
  set -ex; cd "$(mktemp -d)" &&
  if [[ -z ${version} ]]; then version=$(curl -s https://api.github.com/repos/trilioData/tvk-plugins/releases/latest | grep -oP '"tag_name": "\K(.*)(?=")'); fi &&
  echo "Installing version=${version}" &&
  curl -fsSLO "https://github.com/trilioData/tvk-plugins/releases/download/"${version}"/cleanup.tar.gz" &&
  tar zxvf cleanup.tar.gz && sudo mv cleanup/cleanup /usr/local/bin/kubectl-tvk_cleanup
)
```
Verify installation with `kubectl tvk-cleanup --help`

##### Windows
NOT SUPPORTED


## Usage:

```shell script
kubectl tvk-cleanup [options] [arguments]
Options:
        -h, --help                show brief help
        -n, --noninteractive      run script in non-interactive mode
        -c, --crd                 delete Triliovault CRDs
        -t, --tvm                 delete Triliovault Manager or Operator
        -r, --resources \"resource1 resource2..\"
                                  specify list of Triliovault CRs to delete
                                  If not provided, all Triliovault CRs (listed below) will be deleted
                                  e.g. Restore Backup Backupplan ClusterRestore ClusterBackup
                                       ClusterBackupPlan Hook Target Policy License
```

## Examples

- Interactive, Cleans up all:

```shell script
kubectl tvk-cleanup -t -c -r
```

- Non-interactive, Cleans up all:

```shell script
kubectl tvk-cleanup -n -t -c -r
```

- Non-interactive, Cleans up only Triliovault Manager or Operator:

```shell script
kubectl tvk-cleanup -n -t
```

- Non-interactive, Cleans up only Triliovault CRDs

```shell script
kubectl tvk-cleanup -n -c
```

- Non-interactive, Cleans up only specified Triliovault CRs

```shell script
kubectl tvk-cleanup -n -r "Restore Backup Backupplan"
```

