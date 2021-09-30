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

## Installation, Upgrade, Removal of Plugins:


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

## Examples

- To collect logs & YAML from multiple namespaces (separated by commas in double quotes):

        kubectl tvk-log-collector --namespaces "<ns1>,<ns2>" --log-level info

- To collect logs & YAML from all over the cluster:

        kubectl tvk-log-collector --clustered --log-level info

- To collect logs with log level error and to keep the folder with & without zip:

        kubectl tvk-log-collector --clustered --keep-source-folder --log-level error


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