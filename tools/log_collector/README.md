# k8s-triliovault Log Collector
This is a python **pip** module that collects the information mainly yaml configuration and logs from k8s cluster for debugging k8s-triliovault application.

## Requirements
1. python >= 3.6

## How to use
1. ```pip install k8s-triliovault-logcollector --extra-index-url https://pypi.fury.io/k8s-triliovault/``` 
    
2. ```log_collector.py```

Optional arguments: 

| Parameter                 | Default       | Description   |	
| :------------------------ |:-------------:| :-------------|
| --clustered 	       |	false           |whether clustered installtion of trilio application
| --namespaces          | []           |list of namespaces to look for resources
| --kube_config 	       |	~/.kube/config	            |path to the kubernetes config
| --no-clean  		       | false	           | don\'t clean output directory after zip
| --log-level  		       | INFO	           | log level for debugging

## Output
This script will create `triliovault-<date-time>.zip` zip file containing cluster debugging information.

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

