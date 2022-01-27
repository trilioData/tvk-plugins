# Contribution Guidelines

This guide intends to make contribution to this repo easier and consistent.

## Directory Structure:

```text
tvk-plugins
 ├── .github/
 |    └── workflows : github actions workflow files
 |        ├── plugin-manifests.yml : CI workflow for plugin manifest validation
 |        └── plugin-packages.yml : CI workflow for plugin packages(build, test, release)
 ├── cmd : Log-collector, Preflight, Target-Browser executable packages
 ├── .krew : template yamls of plugin manifests(used for update of actual krew manifest and local testing)
 ├── docs : docs of tvk-plugins, contribution and release guidelines
 ├── hack : dir contains helper files for github actions CI workflows
 ├── internal : dir contains funcs to initialize kube-env clients and other helper funcs
 ├── LICENSE.md : License for tvk-plugins
 ├── Makefile : make targets
 ├── plugins : Krew plugin manifests
 │   ├── tvk-log-collector.yaml 
 │   └── tvk-preflight.yaml
 │   ├── tvk-cleanup.yaml 
 │   └── tvk-oneclick.yaml
 │   ├── tvk-target-browser.yaml 
 ├── tests : Integration Test
 │   ├── target-browser : target-browser test suite
 │   └── preflight : preflight test files
 │   ├── cleanup : cleanup test suite
 │   └── tvk-oneclick : oneclick test suite
 ├── tools : business logic of plugins
 │   ├── log-collector : business logic of log-collector
 │   └── preflight : business logic of preflight
 │   └── target-browser : business logic of target-browser CLI
 │   └── cleanup : business logic for cleanup
 ├── .goreleaser.yml : goreleaser conf file(to build & release plugin packages)   
```

## Setup Local Environment

### Pre-requisites:

Ensure following utilities are installed on the local machine:
1. goreleaser 
2. krew
3. kubectl
4. yamllint
5. golangci-lint
6. curl

If these are not installed then, install using `make install` or choose any other installation method to get the latest version. 

### Code Convention:

Run following commands before git push to remote:

```
make ready
```

### Build and Test:

#### Plugin Packages

1. **Preflight**:

    Build: 
    ```
    make build-preflight
    ```
    
    Test:
    ```
    make test-preflight-plugin-locally
    ```
    
    Build and Test together:
    ```
    make test-preflight
    ```

2. **Log-collector**:
     
     Build: 
     ```
     make build-log-collector
     ```

     Test:
     ```
     make test-log-collector-plugin-locally
     ```   
    
     Build and Test together:
     ```
     make test-log-collector
     ```
2. **Target-Browser**:
     
     Build: 
     ```
     make build-target-browser
     ```

     Test:
     ```
     make test-target-browser-integration
     ```   
    
     Build and Test together:
     ```
     make test-target-browser
     ```


3. **All Preflight, Log-collector and Target-Browser together**:

    Build:
    ```
    make build
    ```

    Test: 
    ```
    make test-plugins-locally
    ``` 

    Build and Test together:
    ```
    make test-plugins-packages
    ```
    

#### Plugin Manifests


1. **Update**:
    
    Update plugin manifests kept under [`plugins`](plugins) directory using template yamls from [`.krew`](.krew) directory.
    Need to set `PREFLIGHT_VERSION` & `LOG_COLLECTOR_VERSION` to update plugin manifests.
    
    Set Versions:
    ```
    export PREFLIGHT_VERSION=<preflight-release-tag>
    export LOG_COLLECTOR_VERSION=<log-collector-release-tag>
    export TARGET_BROWSER_VERSION=<target-browser-release-tag>
    export CLEANUP_VERSION=<cleanup-release-tag>
    export TVK_ONECLICK_VERSION=<tvk-oneclick-release-tag>
    ```
   
    Preflight:
    ```
    make update-preflight-manifest
    ```
    
    Log-collector:
    ```
    make update-log-collector-manifest
    ```
   
    Target-Browser:
    ```
    make update-target-browser-manifest
    ```
   
    All Preflight, Log-collector and Target-Browser together:
    ```
    make update-plugin-manifests
    ```

2. **Validate**:

    Validate updated plugin manifests kept under [`plugins`](plugins) directory.
    
    ```
    make validate-plugin-manifests
    ```

#### Run Integration Tests:
   
   For both preflight & target-browser:
   ```
   make test
   ```

```
NOTE: Follow all mentioned code conventions and build & test plugins locally before git push.
```