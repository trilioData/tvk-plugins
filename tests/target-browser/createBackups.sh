#!/bin/bash
# shellcheck disable=SC2004
# shellcheck disable=SC1073
# shellcheck disable=SC1048
# shellcheck disable=SC1072
# shellcheck disable=SC1009

set -ex

src_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixpath="/triliodata"
backupType=('Helm' 'Operator' 'Custom' 'Namespace')

for ((i = 0; i < $1; i++)); do
  bplanuid=$(uuidgen)
  if [ "$5" == "helm_backup_type" ]; then
    index=0
  else
    index=$RANDOM%4
  fi

  for ((j = 0; j < $2; j++)); do

    backupuid=$4
    backuppath=${fixpath}/${bplanuid}/${backupuid}
    completionTime=$(($(($RANDOM % 50)) + 10))
    echo "Creating backup in directory ${backuppath}"
    mkdir -p "${backuppath}"

    # used to add custom parameter values in backup & backupplans .json files
    if [ "$3" = "true" ]; then
      cp "${src_dir}"/test_files/backup-with-placeholders.json "${src_dir}"/test_files/backup-modified.json
      cp "${src_dir}"/test_files/backupplan-with-placeholders.json "${src_dir}"/test_files/backupplan-modified.json

      echo "Replacing placeholders in backup & backupplan json files"
      # change placeholders in backup file with a new values
      sed -i "s/BACKUP-NAME/backup-$j/g" "${src_dir}"/test_files/backup-modified.json
      sed -i "s/BACKUP-UUID/$backupuid/g" "${src_dir}"/test_files/backup-modified.json
      sed -i "s/BACKUPPLAN-UUID/$bplanuid/g" "${src_dir}"/test_files/backup-modified.json
      sed -i "s/BACKUPPLAN-NAME/backupplan-$i/g" "${src_dir}"/test_files/backup-modified.json
      sed -i "s/BACKUP-STATUS/Available/g" "${src_dir}"/test_files/backup-modified.json
      sed -i "s/APPLICATION-TYPE/${backupType[index]}/g" "${src_dir}"/test_files/backup-modified.json
      sed -i "s/COMPLETION-TIMESTAMP/$completionTime/g" "${src_dir}"/test_files/backup-modified.json

      # change placeholders in backupplan file with a new value
      sed -i "s/BACKUP-NAME/backup-$j/g" "${src_dir}"/test_files/backupplan-modified.json
      sed -i "s/BACKUPPLAN-NAME/backupplan-$i/g" "${src_dir}"/test_files/backupplan-modified.json
      sed -i "s/APPLICATION-TYPE/${backupType[index]}/g" "${src_dir}"/test_files/backupplan-modified.json
      sed -i "s/BACKUPPLAN-UUID/$bplanuid/g" "${src_dir}"/test_files/backupplan-modified.json
      sed -i "s/BACKUP-UUID/$backupuid/g" "${src_dir}"/test_files/backupplan-modified.json

      # modify backupcomponents in backupplan json file as per value of index vairiable
      if [[ $index -eq 0 ]]; then
        sed -i "s/\"BACKUPPLAN-COMPONENTS\"/{\"helmReleases\":[\"mysql\"]}/g" "${src_dir}"/test_files/backupplan-modified.json
      elif [[ $index -eq 1 ]]; then
        sed -i "s/\"BACKUPPLAN-COMPONENTS\"/{\"operators\":[{\"operatorId\":\"abc\"}]}/g" "${src_dir}"/test_files/backupplan-modified.json
      elif [[ $index -eq 2 ]]; then
        sed -i "s/\"BACKUPPLAN-COMPONENTS\"/{\"custom\":[{\"matchLabels\":{\"app\":\"nginx\"}}]}/g" "${src_dir}"/test_files/backupplan-modified.json
      else
        sed -i "s/\"BACKUPPLAN-COMPONENTS\"/{}/g" "${src_dir}"/test_files/backupplan-modified.json
      fi

      # copy modified files to NFS location
      mv "${src_dir}"/test_files/backup-modified.json "${backuppath}"/backup.json
      mv "${src_dir}"/test_files/backupplan-modified.json "${backuppath}"/backupplan.json
    else
      if [ "$3" = "mutate-tvk-id" ]; then
        cp "${src_dir}"/test_files/tvk-meta.json "${src_dir}"/test_files/tvk-meta-modified.json
        sed -i "s/TVK-UID/$4/g" "${src_dir}"/test_files/tvk-meta-modified.json
        mv "${src_dir}"/test_files/tvk-meta-modified.json "${backuppath}"/tvk-meta.json
      fi
      cp "${src_dir}"/test_files/backup.json "${backuppath}"/backup.json
      cp "${src_dir}"/test_files/backupplan.json "${backuppath}"/backupplan.json
    fi
  done
done
