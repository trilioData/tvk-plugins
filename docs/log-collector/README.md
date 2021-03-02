# k8s-triliovault Log Collector

Log collector let you define what you need to log and how to log it by collecting the the logs and events of Pod. Pod Logs can help you understand what is happening inside your application. The logs are particularly useful for debugging problems and monitoring cluster activity, alongside the metadata of all resources related to TrilioVault as either namespaced by providing namespaces name separated by comma or clustered from k8s cluster for debugging k8s-triliovault application. It also collects the CRDs yaml related to TVK and zip them.

## Requirements
1. GoLang >= 1.15

## How to use
1. ```go build -o <binary-name> cmd/log-collector/main.go && chmod +x <binary-name>```

2. ```mv <binary-name> /usr/local/bin```
3.  ```<binary-name> [flags]```

Optional arguments:

| Parameter                 | Default       | Description   |	
| :------------------------ |:-------------:| :-------------|
| --clustered 	       |	false           |whether clustered installation of trilio application
| --namespaces          | []           |list of namespaces to look for resources separated by commas
| --kubeconfig 	       |	~/.kube/config	            |path to the kubernetes config
| --no-clean  		       | false	           | don\'t clean output directory after zip
| --log-level  		       | INFO	           | log level for debugging

## Output
This binary will create `triliovault-<date-time>.zip` zip file containing cluster debugging information.

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

