# TVK Target Browser Plugin

**tvk-target-browser** plugin queries content of mounted target location to get details of backup, backupPlan and
metadata details of backup via HTTP/HTTPS calls to target-browser server. Plugin currently supports GET operation on
target-browser's `/backupplan`, `/backup`, `/metadata`, `/resource-metadata`, and `/trilio-resources` API.

## Pre-requisites:

1. krew - kubectl-plugin manager. Install from [here](https://krew.sigs.k8s.io/docs/user-guide/setup/install/)
2. kubectl - kubernetes command-line tool. Install from [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
3. Target CR should have `browsingEnabled` field set to `true` in status
    - CLI Users can check JSON path - `target.status.browsingEnabled`
    - TVK-UI Users should follow steps:
        1. Click on `Resource Management` from the left navigation bar.
        2. In Resource Management, `Backup Plans` tab will be selected by default.
        3. Click on `Targets` tab.
        4. Look for the required target name in the list and ensure that in `Browsing` column toggle is `Enabled` for
           that target.
4. TVK's web-backend service should be up and running.
5. TVK's target-browser ingress path[`spec.rules[*].host`] should be accessible.
    1. If there's no DNS record for ingress-gateway service IP and ingress host path then, create one for it on your local system[`/etc/hosts`].
    2. If ingress-gateway service is `NodePort` type then, make sure NodePort is properly exposed from all cluster nodes.

**Supported OS and Architectures:**

- linux/amd64
- linux/x86
- linux/arm
- linux/arm64
- darwin/amd64
- darwin/arm64
- windows/amd64


## Installation, Upgrade, Removal of Plugins :

#### 1. With `krew`:

  - Add TVK custom plugin index of krew:
  
    ```bash
    kubectl krew index add tvk-plugins https://github.com/trilioData/tvk-plugins.git
    ```
  
  - Installation:
  
    ```bash
    kubectl krew install tvk-plugins/tvk-target-browser
    ```
  
  - Upgrade:
  
    ```bash
    kubectl krew upgrade tvk-target-browser
    ```
  
  - Removal:
  
    ```bash
    kubectl krew uninstall tvk-target-browser
    ```

#### 2. Without `krew`:

1. List of available releases: https://github.com/trilioData/tvk-plugins/releases
2. Choose a version of target-browser plugin to install and check if release assets have target-browser plugin's package
   [target-browser_${version}_${OS}_${ARCH}.tar.gz] for your desired OS & Architecture.
3. Set env variable `version=v1.x.x` [update with your desired version].

##### Linux/macOS

- Bash or ZSH shells
```bash
(
  set -ex; cd "$(mktemp -d)" &&
  OS="$(uname | tr '[:upper:]' '[:lower:]')" &&
  ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')" &&
  package_name="target-browser_${version}_${OS}_${ARCH}.tar.gz" &&
  curl -fsSLO "https://github.com/trilioData/tvk-plugins/releases/download/"${version}"/${package_name}" &&
  tar zxvf ${package_name} && sudo mv target-browser /usr/local/bin/kubectl-tvk_target_browser
)
```
Verify installation with `kubectl tvk-target-browser --help`

##### Windows

1. Download `target-browser_${version}_windows_${ARCH}.zip` from the Releases page to a directory and unzip the package.
2. Launch a command prompt (target-browser.exe).


## Usage:

- Check usage, available commands and flags -

```bash
kubectl tvk-target-browser --help
kubectl tvk-target-browser get --help
kubectl tvk-target-browser get backup --help
kubectl tvk-target-browser get backupPlan --help
kubectl tvk-target-browser get metadata --help
```

## Examples

- Get list of backups:

  ```bash
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --target-name <name> --target-namespace <namespace>
  ```

- Get list of backups using HTTPS:
  ```bash
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --target-name <name> --target-namespace <namespace> --use-https --certificate-authority <certificate-path>
  ```

- Get list of backups for backupPlan:
  ```bash
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --target-name <name> --target-namespace <namespace>
  ```

- Get list of backups for backupPlan using HTTPS:
  ```bash
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --target-name <name> --target-namespace <namespace> --use-https --certificate-authority <certificate-path>
  ```

- Get specific backup:
  ```bash
  kubectl tvk-target-browser get backup <backup-uid> --target-name <name> --target-namespace <namespace>
  ```
- List of backups in Single Namespace:
  ```bash
  kubectl tvk-target-browser get backup --operation-scope SingleNamespace --target-name <name> --target-namespace <namespace>
  ```
- List of backups in Multi Namespace/Cluster Scope:
  ```bash
   kubectl tvk-target-browser get backup --operation-scope MultiNamespace --target-name <name> --target-namespace <namespace>
  ```

- Get list of backupPlans:
  ```bash
  kubectl tvk-target-browser get backupPlan --target-name <name> --target-namespace <namespace>
  ```

- Get specific backupPlan:
  ```bash
  kubectl tvk-target-browser get backupPlan <backup-plan-uid> --target-name <name> --target-namespace <namespace>
  ```
- List of backupPlans in Single Namespace:
  ```bash
  kubectl tvk-target-browser get backupPlan --operation-scope SingleNamespace --target-name <name> --target-namespace <namespace>
  ```
- List of backupPlans in Multi Namespace/Cluster Scope:
  ```bash
  kubectl tvk-target-browser get backupPlan --operation-scope MultiNamespace --target-name <name> --target-namespace <namespace>
  ```

- Get metadata of specific backup:
  ```bash
  kubectl tvk-target-browser get metadata --backup-uid <uid> --backup-plan-uid <uid> --target-name <name> --target-namespace <namespace>
  ```

- Get metadata of specific backup using HTTPS:
  ```bash
  kubectl tvk-target-browser get metadata --backup-uid <uid> --backup-plan-uid <uid> --target-name <name> --target-namespace <namespace> --use-https --certificate-authority <certificate-path>
  ```

- Get resource metadata of specific backup
  ```bash
  kubectl tvk-target-browser get resource-metadata --backup-uid <uid> --target-name <name> --target-namespace <namespace> --group <group> --version <version> --kind <kind> --name <resource-name>
  ```

- Get resource metadata of specific backup and backupPlan
  ```bash
  kubectl tvk-target-browser get resource-metadata --backup-uid <uid> --backup-plan-uid <uid> --target-name <name> --target-namespace <namespace> --group <group> --version <version> --kind <kind> --name <resource-name>
  ```
  
- Get resource metadata of specific backup using HTTPS
  ```bash
  kubectl tvk-target-browser get resource-metadata --backup-uid <uid> --target-name <name> --target-namespace <namespace> --group <group> --version <version> --kind <kind> --name <resource-name> --use-https --certificate-authority <certificate-path>
  ```

- Get trilio resources for specific backup
  ```bash
  kubectl tvk-target-browser get backup trilio-resources <backup-uid> --kinds ClusterBackupPlan,Backup,Hook --target-name <name> --target-namespace <namespace>
  ```

- Get trilio resources for specific backup and backupPlan
  ```bash
  kubectl tvk-target-browser get backup trilio-resources <backup-uid> --backup-plan-uid <uid> --kinds ClusterBackupPlan,Backup,Hook --target-name <name> --target-namespace <namespace>
  ```
  
- List of backups: filter by [expirationStartTime] and [expirationEndTime]
  ```bash
  kubectl tvk-target-browser get backup --expiration-start-time <expiration-start-time> --expiration-end-time <expiration-end-time> --target-name <name> --target-namespace <namespace>
  ```

- List of backups: filter by [creationStartTime] and [creationEndTime]
  ```bash
  kubectl tvk-target-browser get backup --creation-start-time <creation-start-time> --creation-end-time <creation-end-time> --target-name <name> --target-namespace <namespace>
  ```

- List of backupPlans: filter by [creationStartTime] and [creationEndTime]
  ```bash
  kubectl tvk-target-browser get backupPlan --creation-start-time <creation-start-time> --creation-end-time <creation-end-time>--target-name <name> --target-namespace <namespace>
  ```

Find more examples and usage of each command & flag with `--help` for each `tvk-target-browser` command. Refer, `Usage`
section.
