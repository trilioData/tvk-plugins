#!/usr/bin/env bash

set -euo pipefail

# shellcheck disable=SC2046
# shellcheck disable=SC2006
current_tag=$(git describe --abbrev=0 --tags)

# validate if current tag directly references the supplied commit
git describe --exact-match --tags --match "$current_tag"

# TODO: remove this fallback logic one first tag/release is published
# Fallback logic for first tag push, as there'll be no previous tag to compare against
echo "Creating release with both preflight and log-collector packages"
echo "::set-output name=create_release::true"
echo "::set-output name=release_preflight::true"
echo "::set-output name=release_log_collector::true"
exit 0
# fallback logic ends here

# shellcheck disable=SC2046
# shellcheck disable=SC2006
previous_tag=$(git describe --abbrev=0 --tags --match=v[0-9].[0-9].[0-9] --exclude="${current_tag}" --exclude=v*-alpha* --exclude=v*-beta* --exclude=v*-rc* $(git rev-list --tags --skip=1 --max-count=1))

# use hard coded values if required
#current_tag=v0.0.6-main
#previous_tag=v0.0.5-dev

echo "current_tag=$current_tag and previous_tag=$previous_tag"

echo "checking paths of modified files-"

preflight_changed=false
log_collector_changed=false

cmd_dir=cmd
tools_dir=tools
preflight_dir=$tools_dir/preflight
log_collector_dir=$tools_dir/log-collector

# shellcheck disable=SC2086
git diff --name-only $previous_tag $current_tag $tools_dir >files.txt
# shellcheck disable=SC2086
git diff --name-only $previous_tag $current_tag $cmd_dir >>files.txt

count=$(wc -l <files.txt)
if [[ $count -eq 0 ]]; then
  echo "directory 'tools' has not been not modified"
  echo "::set-output name=create_release::false"
  exit
fi

echo "list of modified files-"
cat files.txt

while IFS= read -r file; do
  if [[ $preflight_changed == false && $file == $preflight_dir/* ]]; then
    echo "directory '$preflight_dir' has been modified"
    echo "::set-output name=release_preflight::true"
    preflight_changed=true
  elif [[ ($log_collector_changed == false) && ($file == $log_collector_dir/* || $file == $cmd_dir/*) ]]; then
    echo "directory '$log_collector_dir' or '$cmd_dir' has been modified"
    echo "::set-output name=release_log_collector::true"
    log_collector_changed=true
  fi
done <files.txt

if [[ $preflight_changed == true || $log_collector_changed == true ]]; then
  echo "directory '$tools_dir' has been modified"
  echo "::set-output name=create_release::true"
fi
