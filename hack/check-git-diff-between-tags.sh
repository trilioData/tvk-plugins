#!/usr/bin/env bash

set -euo pipefail

# shellcheck disable=SC2046
# shellcheck disable=SC2006
current_tag=$(git describe --abbrev=0 --tags)

# validate if current tag directly references the supplied commit, this check needed for goreleaser
git describe --exact-match --tags --match "$current_tag"

# shellcheck disable=SC2046
# shellcheck disable=SC2006
# add flag '-v -e "v*-rc*"', if need to compute diff from last stable tag instead of RC
previous_tag=$(git tag --sort=-creatordate | grep -e "v[0-9].[0-9].[0-9]" | grep -v -e "$current_tag" -e "v*-alpha*" -e "v*-beta*" | head -n 1)
# use hard coded values if required
#current_tag=v0.0.6-main
#previous_tag=v0.0.5-dev

echo "current_tag=$current_tag and previous_tag=$previous_tag"

echo "checking paths of modified files-"

preflight_changed=false
log_collector_changed=false
target_browser_changed=false
tvk_oneclick_changed=false
cleanup_changed=false

cmd_dir="cmd"
tools_dir="tools"
log_collector_dir="log-collector"
internal_dir="internal"
target_browser_dir="target-browser"
preflight_dir="preflight"

tvk_oneclick_dir=$tools_dir/tvk-oneclick
cleanup_dir=$tools_dir/cleanup

# shellcheck disable=SC2086
git diff --name-only $previous_tag $current_tag $tools_dir >files.txt
# shellcheck disable=SC2086
git diff --name-only $previous_tag $current_tag $cmd_dir >>files.txt
# shellcheck disable=SC2086
git diff --name-only $previous_tag $current_tag $internal_dir >>files.txt

count=$(wc -l <files.txt)
if [[ $count -eq 0 ]]; then
  echo "no plugin directory has been modified... skipping release"
  echo "::set-output name=create_release::false"
  exit
fi

echo "list of modified files-"
cat files.txt

while IFS= read -r file; do
  if [[ ($preflight_changed == false) && ($file == $internal_dir/* || $file == $tools_dir/$preflight_dir/* || $file == $cmd_dir/$preflight_dir/*) ]]; then
    echo "preflight related code changes have been detected"
    echo "::set-output name=release_preflight::true"
    preflight_changed=true
  fi

  if [[ ($log_collector_changed == false) && ($file == $internal_dir/* || $file == $tools_dir/$log_collector_dir/* || $file == $cmd_dir/$log_collector_dir/*) ]]; then
    echo "log-collector related code changes have been detected"
    echo "::set-output name=release_log_collector::true"
    log_collector_changed=true
  fi

  if [[ ($target_browser_changed == false) && ($file == $internal_dir/* || $file == $tools_dir/$target_browser_dir/* || $file == $cmd_dir/$target_browser_dir/*) ]]; then
    echo "target-browser related code changes have been detected"
    echo "::set-output name=release_target_browser::true"
    target_browser_changed=true
  fi

  if [[ $tvk_oneclick_changed == false && $file == $tvk_oneclick_dir/* ]]; then
    echo "tvk-oneclick related code changes have been detected"
    echo "::set-output name=release_tvk_oneclick::true"
    tvk_oneclick_changed=true
  fi

  if [[ $cleanup_changed == false && $file == $cleanup_dir/* ]]; then
    echo "cleanup related code changes have been detected"
    echo "::set-output name=release_cleanup::true"
    cleanup_changed=true
  fi

done <files.txt

if [[ $preflight_changed == true || $log_collector_changed == true || $target_browser_changed == true || $tvk_oneclick_changed == true || $cleanup_changed ]]; then
  echo "Creating Release as files related to preflight, log-collector, target-browser, tvk-oneclick or cleanup have been changed"
  echo "::set-output name=create_release::true"
fi

# use hard coded values if required for releasing specific plugin package
#echo "Creating release with user defined values"
#echo "::set-output name=create_release::true"
#echo "::set-output name=release_preflight::true"
#echo "::set-output name=release_log_collector::true"
#echo "::set-output name=release_target_browser::true"
#echo "::set-output name=release_tvk_oneclick::true"
#echo "::set-output name=release_cleanup::true"
