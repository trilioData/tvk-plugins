# k8s-triliovault Log Collector

tvk-log-collector collects the logs, config and events of resources. Pod Logs can help you understand what is happening inside your application. The logs are particularly useful for debugging problems and monitoring cluster activity, alongside the metadata of all resources related to TrilioVault as either namespaced by providing namespaces name separated by comma or clustered from k8s cluster for debugging k8s-triliovault application. It also collects the CRDs yaml related to TVK and zip them.

## Pre-requisites:

1. krew - kubectl-plugin manager. Install from [here](https://krew.sigs.k8s.io/docs/user-guide/setup/install/)
2. kubectl - kubernetes command-line tool. Install from [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

**Supported OS/Arch**:

- Linux based x86/x64
- macOS
- Windows


## Installation, Upgrade, Removal of Plugins:


- Add TVK custom plugin index of krew:

  ``` kubectl krew index add tvk-plugins https://github.com/trilioData/tvk-plugins.git```

- Installation:

  ```kubectl krew install tvk-plugins/tvk-log-collector```

- Upgrade:

  ```kubectl krew upgrade tvk-log-collector```

- Removal:

  ```kubectl krew uninstall tvk-log-collector```

- Usage:

  ```kubectl tvk-log-collector [flags]```
- Flags:

| Parameter                 | Default       | Description   |    
| :------------------------ |:-------------:| :-------------|  
| --clustered         |   false           |whether clustered installation of trilio application
| --namespaces          | []           |list of namespaces to look for resources separated by commas
| --kubeconfig            |   ~/.kube/config             |path to the kubernetes config
| --no-clean             | false            | don\'t clean output directory after zip
| --log-level                | INFO             | log level for debugging

## Output
This command will create `triliovault-<date-time>.zip` zip file containing cluster debugging information.

## Resources
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
```  
and ```TrilioVault Resources```