# k8s-triliovault Log Collector
This is a go binary that collects the information mainly yaml configuration and logs from k8s cluster for debugging k8s-triliovault application.

Log collector let you define what you need to log and how to log it by collecting the the logs and events of Pod alongside the metadata of all resources related to TVK as either namespaced by providing namespaces name separated by comma or clustered. It also collects the CRDs related to TVK and zip them on the path you specify

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

