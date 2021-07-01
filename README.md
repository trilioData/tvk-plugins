# tvk-plugins
[![Plugin Packages CI](https://github.com/trilioData/tvk-plugins/actions/workflows/plugin-packages.yml/badge.svg)](https://github.com/trilioData/tvk-plugins/actions/workflows/plugin-packages.yml)
[![Plugin Manifests CI](https://github.com/trilioData/tvk-plugins/actions/workflows/plugin-manifests.yml/badge.svg)](https://github.com/trilioData/tvk-plugins/actions/workflows/plugin-manifests.yml)
[![LICENSE](https://img.shields.io/github/license/trilioData/tvk-plugins.svg)](https://github.com/trilioData/tvk-plugins/blob/master/LICENSE.md)
[![Releases](https://img.shields.io/github/v/release/trilioData/tvk-plugins.svg?include_prereleases)](https://github.com/trilioData/tvk-plugins/releases)

kubectl-plugins for log-collector, preflight and target-browser CLI.

## Pre-requisites:

1. krew - kubectl-plugin manager. Install from [here](https://krew.sigs.k8s.io/docs/user-guide/setup/install/)
2. kubectl - kubernetes command-line tool. Install from [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/)


## Documentation:

Refer [`docs`](docs) for the documentation of preflight and log-collector plugins.
Also, steps to install, uninstall, upgrade and uninstall respective plugin are mentioned in docs.

`Quick Links`: [`preflight`](docs/preflight/README.md)  [`log-collector`](docs/log-collector/README.md) [`target-browser`](docs/target-browser/README.md)

## Contribution:

Refer [`CONTRIBUTION.md`](docs/CONTRIBUTION.md) to make contribution to this repository.

Check plugins manifests [`here`](plugins)

## Release:

Follow release guidelines mentioned in doc [`RELEASE.md`](docs/RELEASE.md)


## LICENSE:

Check here [`LICENSE.md`](LICENSE.md) 
