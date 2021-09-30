# TVK Oneclick Plugin

**tvk-oneclick** is a kubectl plugin which installs,configures and test TVK.
It installs TVK operator,TVK application/manager, configures TVK UI,executes 
some sample backups and restore.

## Pre-requisites:

1. krew - kubectl-plugin manager. Install from [here](https://krew.sigs.k8s.io/docs/user-guide/setup/install/)
2. kubectl - kubernetes command-line tool. Install from [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
3. Helm (version >= 3)
4. Python3(version >= 3.9, with requests package installed - pip3 install requests)
5. S3cmd. Install s3cmd from [here](https://acloud24.com/blog/installation-and-configuration-of-s3cmd-under-linux/)
6. yq(version >= 4). Information can be found @[here](https://github.com/mikefarah/yq) 


**Supported OS:**
- Linux
- darwin

## Performs below tasks

- Preflight:
	It does preflight checks to ensure that all requirements are satisfied.
- Install TVK:
	It installs TVK operator and manager.
- Configure UI:
        It configures TVK UI for user.User has option to select the way in which
        UI should be configured.User can select one from ['Loadbalancer','Nodeport','PortForwarding']
- Create Traget:
	In order to perform sample test, user would require to create target.
        So this option allows user to create S3 or NFS based target.
- Sample Test:
        Plugin allow user to run some sample test. This includes ['Label_based','Namespace_based','Operator_based','Helm_based']
        backup.By default, 'Label_based' backup tests on Mysql application,'Namespace_based' tests on
        wordpress,'Operator_based' tests on postgress operator,'Helm_based' tests using mongodb application.

## Ways in which plugin can be executed

- Interactive:
        Plugin asks various input that requires it to perform the mentioned operations 
        as and when plugin proceeds.
- Non-interactive:
	In this plugin would expect user to provide path and name of config file at the
        start of plugin.
 	sample config file can be found [here](https://github.com/trilioData/tvk-plugins/tree/main/tests/tvk-oneclick/input_config)


## Installation, Upgrade, Removal of Plugins :

- Add TVK custom plugin index of krew:

  ```
  kubectl krew index add tvk-plugins https://github.com/trilioData/tvk-plugins.git
  ```

- Installation:

    ```
    kubectl krew install tvk-plugins/tvk-oneclick
  ```  

- Upgrade:

    ```
    kubectl krew upgrade tvk-oneclick
  ```  

- Removal:

 	```
 	kubectl krew uninstall tvk-oneclick
  ```  

## Usage

tvk-oneclick - Installs, Configures UI, Create sample backup/restore test

Usage:

kubectl tvk-oneclick [options] 

| Parameter                 | Default       | Description   |
| :------------------------ |:-------------:| :-------------|
| --noninteractive          |               |run script in non-interactive mode.for this you need to provide config file
| --install_tvk             |               |Installs TVK and it's free trial license.
| --configure_ui            |               |Configures TVK UI.
| --target                  |		    | Create Target for backup and restore jobs
| --sample_test		    |		    | Create sample backup and restore jobs
| --preflight		    |		    | Checks if all the pre-requisites are satisfied


## Examples

- With `-n`:

```shell script
kubectl tvk-oneclick -n
```

- Without `-n`:

```shell script
kubectl tvk-oneclick -i -c -t -s
```
