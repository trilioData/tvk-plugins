# TVK Log Collector Plugin

tvk-log-collector collects the logs, config and events of resources. Pod Logs can help you understand what is happening inside your application. The logs are particularly useful for debugging problems and monitoring cluster activity, alongside the metadata of all resources related to TrilioVault as either namespaced by providing namespaces name separated by comma or clustered from k8s cluster for debugging k8s-triliovault application. It also collects the CRDs yaml related to TVK and zip them.

## Pre-requisites:

1. krew - kubectl-plugin manager. Install from [here](https://krew.sigs.k8s.io/docs/user-guide/setup/install/)
2. kubectl - kubernetes command-line tool. Install from [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

**Supported OS and Architectures**:
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

  ```
  kubectl krew index add tvk-plugins https://github.com/trilioData/tvk-plugins.git
  ```

- Installation:

  ```
  kubectl krew install tvk-plugins/tvk-log-collector
  ```

- Upgrade:

  ```
  kubectl krew upgrade tvk-log-collector
  ```

- Removal:

  ```
  kubectl krew uninstall tvk-log-collector
  ```

#### 2. Without `krew`:

1. List of available releases: https://github.com/trilioData/tvk-plugins/releases
2. Choose a version of log-collector plugin to install and check if release assets have log-collector plugin's package
   [log-collector_${version}_${OS}_${ARCH}.tar.gz] for your desired OS & Architecture.
   - To check OS & Architecture, execute command `uname -a` on linux/macOS and `systeminfo` on windows
3. Set env variable `version=v1.x.x` [update with your desired version]. If `version` is not exported, `latest` tagged version
   will be considered.

##### Linux/macOS

- Bash or ZSH shells
```bash
(
  set -ex; cd "$(mktemp -d)" &&
  OS="$(uname | tr '[:upper:]' '[:lower:]')" &&
  ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')" &&
  if [[ -z ${version} ]]; then version=$(curl -s https://api.github.com/repos/trilioData/tvk-plugins/releases/latest | grep -oP '"tag_name": "\K(.*)(?=")'); fi &&
  echo "Installing version=${version}" &&
  package_name="log-collector_${version}_${OS}_${ARCH}.tar.gz" &&
  curl -fsSLO "https://github.com/trilioData/tvk-plugins/releases/download/"${version}"/${package_name}" &&
  tar zxvf ${package_name} && sudo mv log-collector /usr/local/bin/kubectl-tvk_log_collector
)
```
Verify installation with `kubectl tvk-log-collector --help`

##### Windows

1. Download `log-collector_${version}_windows_${ARCH}.zip` from the Releases page to a directory and unzip the package.
2. Launch a command prompt (log-collector.exe).


## Usage:

    kubectl tvk-log-collector [flags]

- Flags:

| Parameter                 | Default       | Description   |    
| :------------------------ |:-------------:| :-------------|  
| --clustered         |   false           |whether clustered installation of trilio application
| --namespaces          | []           |list of namespaces to look for resources separated by commas
| --kubeconfig            |   ~/.kube/config             |path to the kubernetes config
| --keep-source-folder            | false            | Keep source directory and Zip both
| --log-level                | INFO             | log level for debugging ( INFO ERROR DEBUG WARNING DEBUG )
| --config-file |  | path to config file for log collector inputs
| --gvk | | json string to give list of GVKs that want be collected other than log collector handles
| --label-selector | | json string to give list of all label selector for resources to be collected other than log collector collects

## Examples

- To collect logs & YAML from multiple namespaces (separated by commas in double quotes):

        kubectl tvk-log-collector --namespaces "<ns1>,<ns2>" --log-level info

- To collect logs & YAML from all over the cluster:

        kubectl tvk-log-collector --clustered --log-level info

- To collect logs with log level error and to keep the folder with & without zip:

        kubectl tvk-log-collector --clustered --keep-source-folder --log-level error

- To collect logs by providing object gvk which log collector doesn't collect by default :

        kubectl tvk-log-collector --clustered --gvks "/v1/pod","apps//Deployment"

- To collect object logs by providing labels which log collector doesn't collect by default :
        
        kubectl tvk-log-collector --clustered  --labels "app=frontend|custom=label","app=backend"

- To collect logs by providing config file :

        kubectl tvk-log-collector --config-file <path/to/config/file.yaml>

The format of data in a file should be according to the below example:

```yaml
keep-source-folder: true
clustered: false
namespaces:
  - default
  - tvk
logLevel: INFO
kubeConfig: path/to/config
labels:
  - matchLabels:
      "app": "frontend"
      "custom": "label"
  - matchLabels:
      "app": "backend"
gvks:
  - group: ""
    version: ""
    kind: pod
  - group: apps
    version: ""
    kind: Deployment

```
Run a log collector with predefined values using a sample file. Download the file using below commands:

By `wget`
```shell script
wget https://github.com/trilioData/tvk-plugins/tree/main/docs/log-collector/sample_input.yaml
```

By `curl`
```shell script
curl https://github.com/trilioData/tvk-plugins/tree/main/docs/log-collector/sample_input.yaml
```

## Output
This command will create `triliovault-<date-time>.zip` zip file containing cluster debugging information.

## Resources Considered for Log Collection:
```  
CustomResourceDefinition  
VolumeSnapshots  
VolumeSnapshotClass  
StorageClass  
Jobs  
Pods  
DaemonSets  
Deployments  
ReplicaSets  
StatefulSet  
PersistentVolumeClaims  
PersistentVolumes  
Services  
ServiceAccounts
Endpoints
Ingress
Events
ConfigMap
LimitRange
ResourceQuota
Role
RoleBinding
Namespaces
Nodes
```
when clustered flag enabled
```
ClusterRole
ClusterRoleBinding
MutatingWebhookConfiguration
ValidatingWebhookConfiguration
PersistentVolume
IngressClass
```
and ```TrilioVault Resources```

## OCP Specific Resources Considered for Log Collection:

```  
ClusterServiceVersion  
CatalogSource
InstallPlan
OperatorCondition
Route
Subscription
```