# tvk-plugins
[![Plugin Packages CI](https://github.com/trilioData/tvk-plugins/actions/workflows/plugin-packages.yml/badge.svg)](https://github.com/trilioData/tvk-plugins/actions/workflows/plugin-packages.yml)
[![Plugin Manifests CI](https://github.com/trilioData/tvk-plugins/actions/workflows/plugin-manifests.yml/badge.svg)](https://github.com/trilioData/tvk-plugins/actions/workflows/plugin-manifests.yml)
[![LICENSE](https://img.shields.io/github/license/trilioData/tvk-plugins.svg)](https://github.com/trilioData/tvk-plugins/blob/master/LICENSE.md)
[![Releases](https://img.shields.io/github/v/release/trilioData/tvk-plugins.svg?include_prereleases)](https://github.com/trilioData/tvk-plugins/releases)

kubectl-plugins for log-collector, preflight and target-browser CLI.

## Pre-requisites:

1. krew - kubectl-plugin manager. Install from [here](https://krew.sigs.k8s.io/docs/user-guide/setup/install/).
2. kubectl - kubernetes command-line tool. Install from [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/).


For openshift environments, if `kubectl` is not installed and `oc` binary is installed on host machine, then `oc` binary
can be used to perform `kubectl` operation by creating symlink with -
```bash
sudo ln -s /usr/local/bin/oc /usr/local/bin/kubectl
```
Note: 
- `oc` binary path can found by executing `which oc`.
- To delete/remove symbolic links use either `unlink` or `rm` command -
```bash
unlink /usr/local/bin/kubectl
```


## Documentation:

Refer [`docs`](docs) for the documentation of all available plugins.
Also, steps to install, uninstall, upgrade and uninstall respective plugin are mentioned in docs.

`Quick Links`: [`preflight`](docs/preflight/README.md)  [`log-collector`](docs/log-collector/README.md) [`target-browser`](docs/target-browser/README.md)
[`cleanup`](docs/cleanup/README.md) [`tvk-oneclick`](docs/tvk-oneclick/README.md) 


#### Plugin Installation with Network Proxy:

If you want to use Krew with proxy(`HTTP`/`HTTPS`), you can configure environment variables HTTP_PROXY, HTTPS_PROXY and NO_PROXY as
required:
```bash
export HTTP_PROXY="http://username:password&proxy-ip:port"
export HTTPS_PROXY="http://username:password&proxy-ip:port"
export NO_PROXY=".github.com,github.com,.githubusercontent.com,githubusercontent.com,ip1,ip2:port2,.example.com"
```

For HTTPS proxy, you'll have to add the proxy server certificate authority's certificates on your local system.
Refer [`document`](https://manuals.gfi.com/en/kerio/connect/content/server-configuration/ssl-certificates/adding-trusted-root-certificates-to-the-server-1605.html)
to do the same.

Refer [document](https://krew.sigs.k8s.io/docs/user-guide/advanced-configuration/#custom-network-proxy) for the latest updates
on krew with proxy setup.

## Contribution:

Refer [`CONTRIBUTION.md`](docs/CONTRIBUTION.md) to make contribution to this repository.

Check plugins manifests [`here`](plugins)

## Release:

Follow release guidelines mentioned in doc [`RELEASE.md`](docs/RELEASE.md)


## LICENSE:

Check here [`LICENSE.md`](LICENSE.md) 
