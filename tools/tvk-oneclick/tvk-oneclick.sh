#!/usr/bin/env bash

#This program is use to install/configure/test TVK product with one click and few required inputs

#This module is used to perform preflight check which checks if all the pre-requisites are satisfied before installing Triliovault for Kubernetes application in a Kubernetes cluster
masterIngName=k8s-triliovault-master
ingressGateway=k8s-triliovault-ingress-gateway

preflight_checks() {
  ret=$(kubectl krew 2>/dev/null)
  if [[ -z "$ret" ]]; then
    echo "Please install krew plugin and then try.For information on krew installation please visit:"
    echo "https://krew.sigs.k8s.io/docs/user-guide/setup/install/"
    return 1
  fi
  ret=$(kubectl tvk-preflight --help 2>/dev/null)
  # shellcheck disable=SC2236
  if [[ ! -z "$ret" ]]; then
    echo "Skipping/Upgrading plugin tvk-preflight installation as it is already installed"
    ret_val=$(kubectl krew upgrade tvk-preflight 2>&1)
    retcode=$?
    if [ "$retcode" -ne 0 ]; then
      echo "$ret_val" | grep -q "can't upgrade, the newest version is already installed"
      ret=$?
      if [ "$ret" -ne 0 ]; then
        echo "Failed to uggrade tvk-plugins/tvk-preflight plugin"
        return 1
      else
        echo "tvk-preflight is already the newest version"
      fi
    fi
  else
    plugin_url='https://github.com/trilioData/tvk-plugins.git'
    kubectl krew index add tvk-plugins "$plugin_url" 1>> >(logit) 2>> >(logit)
    kubectl krew install tvk-plugins/tvk-preflight 1>> >(logit) 2>> >(logit)
    retcode=$?
    if [ "$retcode" -ne 0 ]; then
      echo "Failed to install tvk-plugins/tvk-preflight plugin" 2>> >(logit)
    fi
  fi
  if [[ -z "${input_config}" ]]; then
    read -r -p "Provide storageclass to be used for TVK/Application Installation (storageclass with default annotation will be take as default): " storage_class
  fi
  if [[ -z "$storage_class" ]]; then
    storage_class=$(kubectl get storageclass | grep -w '(default)' | awk '{print $1}')
    if [[ -z "$storage_class" ]]; then
      echo "No default storage class found, need one to proceed"
      return 1
    fi
  fi
  check=$(kubectl tvk-preflight --storageclass "$storage_class" | tee /dev/tty)
  ret_code=$?
  if [ "$ret_code" -ne 0 ]; then
    echo "Failed to run 'kubectl tvk-preflight',please check if PATH variable for krew is set properly and then try"
  fi
  check_for_fail=$(echo "$check" | grep 'Some Pre-flight Checks Failed!')
  if [[ -z "$check_for_fail" ]]; then
    echo "All preflight checks are done and you can proceed"
  else
    if [[ -z "${input_config}" ]]; then
      echo "There are some failures"
      read -r -p "Do you want to proceed? y/n: " proceed_even_PREFLIGHT_fail
    fi
    if [[ "$proceed_even_PREFLIGHT_fail" != "Y" ]] && [[ "$proceed_even_PREFLIGHT_fail" != "y" ]]; then
      exit 1
    fi
  fi
}

#This function is use to compare 2 versions
vercomp() {
  if [[ $1 == "$2" ]]; then
    return 0
  fi
  ret2=$(python3 -c "from packaging import version;print(version.parse(\"$1\") < version.parse(\"$2\"))")
  ret1=$(python3 -c "from packaging import version;print(version.parse(\"$1\") == version.parse(\"$2\"))")
  if [[ $ret2 == "True" ]]; then
    return 2
  elif [[ $ret1 == "True" ]]; then
    return 1
  else
    return 3
  fi
  return 0
}

#function to print waiting symbol
wait_install() {
  runtime=$1
  spin='-\|/'
  i=0
  #endtime=$(date -ud "$runtime" +%s)
  endtime=$(python3 -c "import time;timeout = int(time.time()) + 60*$runtime;print(\"{0}\".format(timeout))")
  if [[ -z ${endtime} ]]; then
    echo "There is some issue with date usage, please check the pre-requsites in README page" 1>> >(logit) 2>> >(logit)
    echo "Something went wrong..terminating" 2>> >(logit)
  fi
  val1=$(eval "$2")
  while [[ $(python3 -c "import time;timeout = int(time.time());print(\"{0}\".format(timeout))") -le $endtime ]] && [[ "" == "$val1" ]] || [[ "$val1" == '{}' ]] || [[ "$val1" == 'map[]' ]]; do
    i=$(((i + 1) % 4))
    printf "\r %s" "${spin:$i:1}"
    sleep .1
    val1=$(eval "$2")
  done
  echo ""
}

#This module is used to install TVK along with its free trial license
install_tvk() {
  # Add helm repo and install triliovault-operator chart
  helm repo add triliovault-operator http://charts.k8strilio.net/trilio-stable/k8s-triliovault-operator 1>> >(logit) 2>> >(logit)
  retcode=$?
  if [ "$retcode" -ne 0 ]; then
    echo "There is some error in helm update,please resolve and try again" 1>> >(logit) 2>> >(logit)
    echo "Error ading helm repo"
    return 1
  fi
  helm repo add triliovault http://charts.k8strilio.net/trilio-stable/k8s-triliovault 1>> >(logit) 2>> >(logit)
  helm repo update 1>> >(logit) 2>> >(logit)
  if [[ -z ${input_config} ]]; then
    read -r -p "Please provide the operator version to be installed (default - 2.6.0): " operator_version
    read -r -p "Please provide the triliovault manager version (default - 2.6.1): " triliovault_manager_version
    read -r -p "Namespace name in which TVK should be installed: (default - default): " tvk_ns
    read -r -p "Proceed even if resource exists y/n (default - y): " if_resource_exists_still_proceed
  fi
  if [[ -z "$if_resource_exists_still_proceed" ]]; then
    if_resource_exists_still_proceed='y'
  fi
  if [[ -z "$operator_version" ]]; then
    operator_version='2.6.0'
  fi
  if [[ -z "$triliovault_manager_version" ]]; then
    triliovault_manager_version='2.6.1'
  fi
  if [[ -z "$tvk_ns" ]]; then
    tvk_ns="default"
  fi
  get_ns=$(kubectl get deployments -l "release=triliovault-operator" -A 2>> >(logit) | awk '{print $1}' | sed -n 2p)
  if [ -z "$get_ns" ]; then
    #Create ns for installation, if not there.
    ret=$(kubectl get ns $tvk_ns 2>/dev/null)
    if [[ -z "$ret" ]]; then
      if ! kubectl create ns $tvk_ns 2>> >(logit); then
        echo "$tvk_ns namespace creation failed"
        return 1
      fi
    fi
    # Install triliovault operator
    echo "Installing Triliovault operator..."
    helm install triliovault-operator triliovault-operator/k8s-triliovault-operator --version $operator_version -n $tvk_ns 2>> >(logit)
    retcode=$?
    if [ "$retcode" -ne 0 ]; then
      echo "There is some error in helm install triliovaul operator,please resolve and try again" 2>> >(logit)
      return 1
    fi
    get_ns=$(kubectl get deployments -l "release=triliovault-operator" -A 2>> >(logit) | awk '{print $1}' | sed -n 2p)
  else
    tvk_ns="$get_ns"
    echo "Triliovault operator is already installed!"
    if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
      exit 1
    fi
    old_operator_version=$(helm list -n "$get_ns" | grep k8s-triliovault-operator | awk '{print $9}' | rev | cut -d- -f1 | rev | sed 's/[a-z-]//g')
    # shellcheck disable=SC2001
    new_operator_version=$(echo $operator_version | sed 's/[a-z-]//g')
    vercomp "$old_operator_version" "$new_operator_version"
    ret_val=$?
    if [[ $ret_val != 2 ]]; then
      echo "Triliovault operator cannot be upgraded, please check version number"
      if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
        exit 1
      fi
    else
      upgrade_tvo=1
      echo "Upgrading Triliovault operator"
      # shellcheck disable=SC2206
      semver=(${old_operator_version//./ })
      major="${semver[0]}"
      minor="${semver[1]}"
      sub_ver=${major}.${minor}
      if [[ $sub_ver == 2.0 ]]; then
        helm plugin install https://github.com/trilioData/tvm-helm-plugins >/dev/null 1>> >(logit) 2>> >(logit)
        rel_name=$(helm list | grep k8s-triliovault-operator | awk '{print $1}')
        helm tvm-upgrade --release="$rel_name" --namespace="$get_ns" 2>> >(logit)
        retcode=$?
        if [ "$retcode" -ne 0 ]; then
          echo "There is some error in helm tvm-upgrade,please resolve and try again" 2>> >(logit)
          return 1
        fi
      fi
      helm upgrade triliovault-operator triliovault-operator/k8s-triliovault-operator --version $operator_version -n "$get_ns" 2>> >(logit)
      retcode=$?
      if [ "$retcode" -ne 0 ]; then
        echo "There is some error in helm upgrade,please resolve and try again" 2>> >(logit)
        return 1
      fi
      sleep 10
    fi
  fi
  cmd="kubectl get pod -l release=triliovault-operator -n $tvk_ns -o 'jsonpath={.items[*].status.conditions[*].status}' | grep -v False"
  wait_install 10 "$cmd"
  if ! kubectl get pods -l release=triliovault-operator -n "$tvk_ns" 2>/dev/null | grep -q Running; then
    if [[ $upgrade_tvo == 1 ]]; then
      echo "Triliovault operator upgrade failed"
    else
      echo "Triliovault operator installation failed"
    fi
    return 1
  fi
  echo "Triliovault operator is running"
  #set value for tvm_name
  tvm_name="triliovault-manager"
  #check if TVK manager is installed
  ret_code=$(kubectl get tvm -A 2>/dev/null)
  if [[ -n "$ret_code" ]]; then
    echo "Triliovault manager is already installed"
    if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
      exit 1
    fi
    tvm_name=$(kubectl get tvm -A | awk '{print $2}' | sed -n 2p)
    tvk_ns="$get_ns"
    #Check if TVM can be upgraded
    old_tvm_version=$(kubectl get TrilioVaultManager -n "$get_ns" -o json | grep releaseVersion | awk '{print$2}' | sed 's/[a-z-]//g' | sed -e 's/^"//' -e 's/"$//')
    # shellcheck disable=SC2001
    new_triliovault_manager_version=$(echo $triliovault_manager_version | sed 's/[a-z-]//g')
    vercomp "$old_tvm_version" "$new_triliovault_manager_version"
    ret_val=$?
    if [[ $ret_val != 2 ]]; then
      echo "TVM cannot be upgraded! Please check version"
      if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
        exit 1
      fi
      install_license "$tvk_ns"
      return
    else
      echo "checking if triliovault manager can be upgraded"
      tvm_upgrade=1
      vercomp "2.5" "$new_triliovault_manager_version"
      ret_val=$?
      vercomp "2.6" "$new_triliovault_manager_version"
      ret_val1=$?
      vercomp "$old_tvm_version" "2.5"
      ret_val2=$?
      if [[ $ret_val == 2 ]] || [[ $ret_val == 1 ]] && [[ $ret_val1 == 3 ]] && [[ $ret_val2 == 2 ]] || [[ $ret_val2 == 1 ]]; then
        svc_type=$(kubectl get svc "$ingressGateway" -n "$get_ns" -o 'jsonpath={.spec.type}')
        if [[ $svc_type == LoadBalancer ]]; then
          get_host=$(kubectl get ingress k8s-triliovault-ingress-master -n "$get_ns" -o 'jsonpath={.spec.rules[0].host}')
          cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: TrilioVaultManager
metadata:
  labels:
    triliovault: triliovault
  name: ${tvm_name}
  namespace: ${tvk_ns}
spec:
  trilioVaultAppVersion: ${triliovault_manager_version}
  componentConfiguration:
    ingress-controller:
      service:
        type: LoadBalancer
      host: "${get_host}"
  helmVersion:
    version: v3
  applicationScope: Cluster
EOF
          retcode=$?
          if [ "$retcode" -ne 0 ]; then
            echo "There is error upgrading triliovault manager,please resolve and try again" 2>> >(logit)
            return 1
          else
            echo "Upgrading Triliovault manager"
          fi
        fi
      elif [[ $ret_val1 == 2 ]] || [[ $ret_val1 == 1 ]] && [[ $ret_val2 == 2 ]] || [[ $ret_val2 == 1 ]]; then
        svc_type=$(kubectl get svc "$ingressGateway" -n "$get_ns" -o 'jsonpath={.spec.type}')
        if [[ $svc_type == LoadBalancer ]]; then
          get_host=$(kubectl get ingress k8s-triliovault-ingress-master -n "$get_ns" -o 'jsonpath={.spec.rules[0].host}')
          cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: TrilioVaultManager
metadata:
  labels:
    triliovault: triliovault
  name: ${tvm_name}
  namespace: ${tvk_ns}
spec:
  trilioVaultAppVersion: ${triliovault_manager_version}
  ingressConfig:
    host: "${get_host}"
  # TVK components configuration, currently supports control-plane, web, exporter, web-backend, ingress-controller, admission-webhook.
  # User can configure resources for all componentes and can configure service type and host for the ingress-controller
  componentConfiguration:
    ingress-controller:
      service:
        type: LoadBalancer
  applicationScope: Cluster
EOF
          retcode=$?
          if [ "$retcode" -ne 0 ]; then
            echo "There is error upgrading triliovault manager,please resolve and try again" 2>> >(logit)
            return 0
          else
            echo "Upgrading Triliovault manager"
          fi
        fi
      elif [[ $ret_val1 == 2 ]] || [[ $ret_val1 == 1 ]]; then
        svc_type=$(kubectl get svc "$ingressGateway" -n "$get_ns" -o 'jsonpath={.spec.type}')
        if [[ $svc_type == LoadBalancer ]]; then
          get_host=$(kubectl get ingress "$masterIngName" -n "$get_ns" -o 'jsonpath={.spec.rules[0].host}')
          cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: TrilioVaultManager
metadata:
  labels:
    triliovault: triliovault
  name: ${tvm_name}
  namespace: ${tvk_ns}
spec:
  trilioVaultAppVersion: ${triliovault_manager_version}
  ingressConfig:
    host: "${get_host}"
  # TVK components configuration, currently supports control-plane, web, exporter, web-backend, ingress-controller, admission-webhook.
  # User can configure resources for all componentes and can configure service type and host for the ingress-controller
  componentConfiguration:
    ingress-controller:
      service:
        type: LoadBalancer
  applicationScope: Cluster
EOF
        elif [[ $svc_type == NodePort ]]; then
          get_host=$(kubectl get ingress "$masterIngName" -n "$get_ns" -o 'jsonpath={.spec.rules[0].host}')
          cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: TrilioVaultManager
metadata:
  labels:
    triliovault: triliovault
  name: ${tvm_name}
  namespace: ${tvk_ns}
spec:
  trilioVaultAppVersion: ${triliovault_manager_version}
  ingressConfig:
    host: "${get_host}"
  # TVK components configuration, currently supports control-plane, web, exporter, web-backend, ingress-controller, admission-webhook.
  # User can configure resources for all componentes and can configure service type and host for the ingress-controller
  componentConfiguration:
    ingress-controller:
      service:
        type: NodePort
  applicationScope: Cluster
EOF
        fi

        retcode=$?
        if [ "$retcode" -ne 0 ]; then
          echo "There is error upgrading triliovault manager,please resolve and try again" 2>> >(logit)
          return 1
        else
          echo "Upgrading Triliovault manager"
        fi
      fi
    fi
  else
    # Create TrilioVaultManager CR
    sleep 10
    vercomp "2.6" "$new_triliovault_manager_version"
    ret_val=$?
    if [[ $ret_val == 2 ]] || [[ $ret_val == 1 ]] && [[ $tvm_upgrade != 1 ]]; then

      cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: TrilioVaultManager
metadata:
  labels:
    triliovault: k8s
  name: ${tvm_name}
  namespace: ${tvk_ns}
spec:
  trilioVaultAppVersion: ${triliovault_manager_version}
  applicationScope: Cluster
  # TVK components configuration, currently supports control-plane, web, exporter, web-backend, ingress-controller, admission-webhook.
  # User can configure resources for all componentes and can configure service type and host for the ingress-controller
  componentConfiguration:
    web-backend:
      resources:
        requests:
          memory: "400Mi"
          cpu: "200m"
        limits:
          memory: "2584Mi"
          cpu: "1000m"
    ingress-controller:
      service:
        type: LoadBalancer
      host: "trilio.co.us"
EOF
      retcode=$?
      if [ "$retcode" -ne 0 ]; then
        echo "There is error in installingi/upgrading triliovault manager,please resolve and try again" 2>> >(logit)
        return 1
      fi
    else
      cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: TrilioVaultManager
metadata:
  labels:
    triliovault: triliovault
  name: ${tvm_name}
  namespace: ${tvk_ns}
spec:
  trilioVaultAppVersion: ${triliovault_manager_version}
  helmVersion:
    version: v3
  applicationScope: Cluster
EOF
      retcode=$?
      if [ "$retcode" -ne 0 ]; then
        echo "There is error in installingi/upgrading triliovault manager,please resolve and try again" 2>> >(logit)
        return 1
      fi
    fi
  fi
  sleep 5
  if [[ $tvm_upgrade == 1 ]]; then
    echo "Waiting for Pods to come up.."
  else
    echo "Installing Triliovault manager...."
  fi
  cmd="kubectl get pods -l app=k8s-triliovault-control-plane -n $tvk_ns 2>/dev/null | grep Running"
  wait_install 10 "$cmd"
  cmd="kubectl get pods -l app=k8s-triliovault-admission-webhook -n $tvk_ns 2>/dev/null | grep Running"
  wait_install 10 "$cmd"
  if ! kubectl get pods -l app=k8s-triliovault-control-plane -n "$tvk_ns" 2>/dev/null | grep -q Running && ! kubectl get pods -l app=k8s-triliovault-admission-webhook -n "$tvk_ns" 2>/dev/null | grep -q Running; then
    if [[ $tvm_upgrade == 1 ]]; then
      echo "TVM upgrade failed"
    else
      echo "TVM installation failed"
    fi
    return 1
  fi
  if [[ $tvm_upgrade == 1 ]]; then
    echo "TVM is upgraded successfully!"
  else
    echo "TVK Manager is installed"
  fi
  install_license "$tvk_ns"
}

#This module is use to install license
install_license() {
  tvk_ns=$1
  flag=0
  ret=$(kubectl get license -n "$tvk_ns" 2>> >(logit) | awk '{print $1}' | sed -n 2p)
  if [[ -n "$ret" ]]; then
    ret_val=$(kubectl get license "$ret" -n "$get_ns" 2>> >(logit) | grep -q Active)
    ret_code_A=$?
    if [ "$ret_code_A" -eq 0 ]; then
      echo "License is already installed and is in active state"
      return
    fi
    #license is installed but is in inactive state
    echo "License is already installed and is in inactive state"
    flag=1
  fi

  echo "Installing required packages.."
  {
    pip3 install requests
    pip3 install beautifulsoup4
    pip3 install lxml
    pip3 install yaml

  } 1>> >(logit) 2>> >(logit)
  echo "Installing Freetrial license..."
  cat <<EOF | python3
#!/usr/bin/env python3
from bs4 import BeautifulSoup
import sys
import subprocess
import warnings
import yaml
warnings.filterwarnings("ignore")
import requests
headers = {'Content-type': 'application/x-www-form-urlencoded; charset=utf-8'}
endpoint="https://doc.trilio.io:5000/8d92edd6-514d-4acd-90f6-694cb8d83336/0061K00000fwkzU"
result = subprocess.check_output("kubectl get ns kube-system -o=jsonpath='{.metadata.uid}'", shell=True)
kubeid = result.decode("utf-8")
data = "kubescope=clusterscoped&kubeuid={0}".format(kubeid)
r = requests.post(endpoint, data=data, headers=headers)
contents=r.content
soup = BeautifulSoup(contents, 'lxml')
sys.stdout = open("license_file1.yaml", "w")
print(soup.body.find('div', attrs={'class':'yaml-content'}).text)
sys.stdout.close()
if($flag == 1):
  with open('license_file1.yaml') as f:
    doc = yaml.safe_load(f)
  doc['metadata']['name'] = "$ret"

  with open('license_file1.yaml', 'w') as f:
    yaml.dump(doc, f)

result = subprocess.check_output("kubectl apply -f license_file1.yaml -n $tvk_ns", shell=True)
EOF
  cmd="kubectl get license -n $tvk_ns 2>> >(logit) | awk '{print $2}' | sed -n 2p | grep Active"
  wait_install 5 "$cmd"
  ret=$(kubectl get license -n "$tvk_ns" 2>> >(logit) | grep -q Active)
  ret_code=$?
  if [ "$ret_code" -ne 0 ]; then
    echo "License installation failed"
    exit 1
  else
    echo "License is installed successfully"
  fi
  rm -f license_file1.yaml
}

#This module is used to configure TVK UI
configure_ui() {
  if [[ -z ${input_config} ]]; then
    echo -e "TVK UI can be accessed using \n1.LoadBalancer \n2.NodePort \n3.PortForwarding"
    read -r -p "Please enter option: " ui_access_type
  else
    if [[ $ui_access_type == 'Loadbalancer' ]]; then
      ui_access_type=1
    elif [[ $ui_access_type == 'Nodeport' ]]; then
      ui_access_type=2
    elif [[ $ui_access_type == 'PortForwarding' ]]; then
      ui_access_type=3
    else
      echo "Wrong option selected for ui_access_type"
      return 1
    fi
  fi
  if [[ -z "$ui_access_type" ]]; then
    ui_access_type=2
  fi
  case $ui_access_type in
  3)
    get_ns=$(kubectl get deployments -l "release=triliovault-operator" -A 2>> >(logit) | awk '{print $1}' | sed -n 2p)
    echo "kubectl port-forward --address 0.0.0.0 svc/$ingressGateway -n $get_ns 80:80 &"
    echo "Copy & paste the command above into your terminal session and TVK management console traffic will be forwarded to your localhost IP of 127.0.0.1 via port 80."
    ;;
  2)
    configure_nodeport_for_tvkui
    return 0
    ;;
  1)
    configure_loadbalancer_for_tvkUI
    return 0
    ;;
  *)
    echo "Incorrect choice"
    return
    ;;
  esac
  shift
}

#This function is used to configure TVK UI through nodeport
configure_nodeport_for_tvkui() {
  ret=$(doctl auth list 2>/dev/null)
  if [[ -z $ret ]]; then
    echo "This functionality requires doctl installed"
    echo "Please follow  to install https://docs.digitalocean.com/reference/doctl/how-to/install/ doctl"
    return 1
  fi
  if [[ -z ${input_config} ]]; then
    read -r -p "Please enter host name for tvk ingress (default - tvk-doks.com): " tvkhost_name
  fi
  if [[ -z ${tvkhost_name} ]]; then
    tvkhost_name="tvk-doks.com"
  fi
  get_ns=$(kubectl get deployments -l "release=triliovault-operator" -A 2>> >(logit) | awk '{print $1}' | sed -n 2p)
  # shellcheck disable=SC1083
  gateway=$(kubectl get pods --no-headers=true -n "$get_ns" 2>/dev/null | awk "/$ingressGateway/"{'print $1}')
  if [[ -z "$gateway" ]]; then
    echo "Not able to find $ingressGateway resource,TVK UI configuration failed"
    return 1
  fi
  node=$(kubectl get pods "$gateway" -n "$get_ns" -o jsonpath='{.spec.nodeName}' 2>> >(logit))
  ip=$(kubectl get node "$node" -n "$get_ns" -o jsonpath='{.status.addresses[?(@.type=="ExternalIP")].address}' 2>> >(logit))
  port=$(kubectl get svc "$ingressGateway" -n "$get_ns" -o jsonpath='{.spec.ports[?(@.name=="http")].nodePort}' 2>> >(logit))
  # Getting tvm version and setting the configs accordingly
  tvm_name=$(kubectl get tvm -A | awk '{print $2}' | sed -n 2p)
  tvk_ns=$(kubectl get tvm -A | awk '{print $1}' | sed -n 2p)
  tvm_version=$(kubectl get TrilioVaultManager -n "$get_ns" -o json | grep releaseVersion | awk '{print$2}' | sed 's/[a-z-]//g' | sed -e 's/^"//' -e 's/"$//')
  vercomp "2.6.0" "$tvm_version"
  ret_val=$?
  if [[ $ret_val == 2 ]] || [[ $ret_val == 1 ]]; then
    retry=5
    while [[ $retry -gt 0 ]]; do
      cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: TrilioVaultManager
metadata:
  labels:
    triliovault: k8s
  name: $tvm_name
  namespace: $tvk_ns
spec:
  applicationScope: Cluster
  ingressConfig:
    host: ${tvkhost_name}
  # TVK components configuration, currently supports control-plane, web, exporter, web-backend, ingress-controller, admission-webhook.
  # User can configure resources for all componentes and can configure service type and host for the ingress-controller
  componentConfiguration:
    ingress-controller:
      service:
        type: NodePort
EOF
      ret_code=$?
      if [[ "$ret_code" -eq 0 ]]; then
        break
      else
        retry="$((retry - 1))"
      fi
    done
    if [[ "$ret_code" -ne 0 ]]; then
      echo "Error while configuring TVM CRD.."
      return 1
    fi
  else
    if ! kubectl patch ingress k8s-triliovault-ingress-master -n "$get_ns" -p '{"spec":{"rules":[{"host":"'"${tvkhost_name}"'"}]}}'; then
      echo "TVK UI configuration failed, please check ingress"
      return 1
    fi
    if ! kubectl patch svc "$ingressGateway" -n "$get_ns" -p '{"spec": {"type": "NodePort"}}' 1>> >(logit) 2>> >(logit); then
      echo "TVK UI configuration failed, please check ingress"
      return 1
    fi
  fi
  cluster_name=$(kubectl config view --minify -o jsonpath='{.clusters[].name}' | cut -d'-' -f3-)
  doctl kubernetes cluster kubeconfig show "${cluster_name}" >config_"${cluster_name}" 2>> >(logit)
  echo "Please add '$ip $tvkhost_name' entry to your /etc/host file before launching the console"
  echo "After creating an entry,TVK UI can be accessed through http://$tvkhost_name:$port/login"
  echo "provide config file stored at location: $PWD/config_${cluster_name}"
  echo "For https access, please refer - https://docs.trilio.io/kubernetes/management-console/user-interface/accessing-the-ui"
}

#This function is used to configure TVK UI through Loadbalancer
configure_loadbalancer_for_tvkUI() {
  ret=$(doctl auth list 2>/dev/null)
  if [[ -z $ret ]]; then
    echo "This functionality requires doctl installed"
    echo "Please follow  to install https://docs.digitalocean.com/reference/doctl/how-to/install/ doctl"
    return 1
  fi
  if [[ -z ${input_config} ]]; then
    echo "To use DigitalOcean DNS, you need to register a domain name with a registrar and update your domain’s NS records to point to DigitalOcean’s name servers."
    read -r -p "Please enter domainname for cluster (Domain name you have registered and added in Doks console): " domain
    read -r -p "Please enter host name for tvk ingress (default - tvk-doks): " tvkhost_name
    read -r -p "Please enter auth token for doctl: " doctl_token
  fi
  if [[ -z ${doctl_token} ]]; then
    echo "This functionality requires Digital Ocean authentication token"
    return 1
  fi
  if [[ -z ${tvkhost_name} ]]; then
    tvkhost_name="tvk-doks"
  fi
  ret=$(doctl auth init -t "$doctl_token")
  ret_code=$?
  if [ "$ret_code" -ne 0 ]; then
    echo "Cannot authenticate with the provided doctl auth token"
    return 1
  fi
  get_ns=$(kubectl get deployments -l "release=triliovault-operator" -A 2>> >(logit) | awk '{print $1}' | sed -n 2p)
  cluster_name=$(kubectl config view --minify -o jsonpath='{.clusters[].name}' | cut -d'-' -f3-)
  if [[ -z ${cluster_name} ]]; then
    echo "Error in getting cluster name from the current-context set in kubeconfig"
    echo "Please check the current-context"
    return 1
  fi
  # Getting tvm version and setting the configs accordingly
  tvm_name=$(kubectl get tvm -A | awk '{print $2}' | sed -n 2p)
  tvk_ns=$(kubectl get tvm -A | awk '{print $1}' | sed -n 2p)
  tvm_version=$(kubectl get TrilioVaultManager -n "$get_ns" -o json | grep releaseVersion | awk '{print$2}' | sed 's/[a-z-]//g' | sed -e 's/^"//' -e 's/"$//')
  vercomp "2.6.0" "$tvm_version"
  ret_val=$?
  if [[ $ret_val == 2 ]] || [[ $ret_val == 1 ]]; then
    retry=5
    while [[ $retry -gt 0 ]]; do
      cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: TrilioVaultManager
metadata:
  labels:
    triliovault: k8s
  name: $tvm_name
  namespace: $tvk_ns
spec:
  applicationScope: Cluster
  ingressConfig:
    host: ${tvkhost_name}.${domain}
  # TVK components configuration, currently supports control-plane, web, exporter, web-backend, ingress-controller, admission-webhook.
  # User can configure resources for all componentes and can configure service type and host for the ingress-controller
  componentConfiguration:
    ingress-controller:
      service:
        type: LoadBalancer
EOF
      ret_code=$?
      if [[ "$ret_code" -eq 0 ]]; then
        break
      else
        retry="$((retry - 1))"
      fi
    done
    if [[ "$ret_code" -ne 0 ]]; then
      echo "Error while configuring TVM CRD.."
      return 1
    fi
  else
    if ! kubectl patch svc "$ingressGateway" -n "$get_ns" -p '{"spec": {"type": "LoadBalancer"}}' 1>> >(logit) 2>> >(logit); then
      echo "TVK UI configuration failed, please check ingress"
      return 1
    fi
  fi
  echo "Configuring UI......This may take some time"
  cmd="kubectl get svc $ingressGateway -n $get_ns -o 'jsonpath={.status.loadBalancer}'"
  wait_install 20 "$cmd"
  val_status=$(kubectl get svc "$ingressGateway" -n "$get_ns" -o 'jsonpath={.status.loadBalancer}')
  if [[ $val_status == '{}' ]] || [[ $val_status == 'map[]' ]]; then
    echo "Loadbalancer taking time to get External IP"
    return 1
  fi
  external_ip=$(kubectl get svc "$ingressGateway" -n "$get_ns" -o 'jsonpath={.status.loadBalancer.ingress[0].ip}' 2>> >(logit))
  if [[ $ret_val != 2 ]] && [[ $ret_val != 1 ]]; then
    kubectl patch ingress k8s-triliovault-ingress-master -n "$get_ns" -p '{"spec":{"rules":[{"host":"'"${tvkhost_name}.${domain}"'"}]}}' 1>> >(logit) 2>> >(logit)
  fi
  doctl compute domain records create "${domain}" --record-type A --record-name "${tvkhost_name}" --record-data "${external_ip}" 1>> >(logit) 2>> >(logit)
  retCode=$?
  if [[ "$retCode" -ne 0 ]]; then
    echo "Failed to create record, please check domain name"
    return 1
  fi

  doctl kubernetes cluster kubeconfig show "${cluster_name}" >config_"${cluster_name}" 2>> >(logit)
  link="http://${tvkhost_name}.${domain}/login"
  echo "You can access TVK UI: $link"
  echo "provide config file stored at location: $PWD/config_${cluster_name}"
  echo "Info:UI may take 30 min to come up"
}

call_s3cfg_doks() {
  access_key=$1
  secret_key=$2
  host_base=$3
  host_bucket=$4
  gpg_passphrase=$5

  cat >s3cfg_config <<-EOM
[default]
access_key = ${access_key}
access_token =
add_encoding_exts =
add_headers =
bucket_location = US
ca_certs_file =
cache_file =
check_ssl_certificate = True
check_ssl_hostname = True
cloudfront_host = cloudfront.amazonaws.com
default_mime_type = binary/octet-stream
delay_updates = False
delete_after = False
delete_after_fetch = False
delete_removed = False
dry_run = False
enable_multipart = True
encoding = UTF-8
encrypt = False
expiry_date =
expiry_days =
expiry_prefix =
follow_symlinks = False
force = False
get_continue = False
gpg_command = /usr/bin/gpg
gpg_decrypt = %(gpg_command)s -d --verbose --no-use-agent --batch --yes --passphrase-fd %(passphrase_fd)s -o %(output_file)s %(input_file)s
gpg_encrypt = %(gpg_command)s -c --verbose --no-use-agent --batch --yes --passphrase-fd %(passphrase_fd)s -o %(output_file)s %(input_file)s
gpg_passphrase = ${gpg_passphrase}
guess_mime_type = True
host_base = ${host_base}
host_bucket = ${host_bucket}
human_readable_sizes = False
invalidate_default_index_on_cf = False
invalidate_default_index_root_on_cf = True
invalidate_on_cf = False
kms_key =
limit = -1
limitrate = 0
list_md5 = False
log_target_prefix =
long_listing = False
max_delete = -1
mime_type =
multipart_chunk_size_mb = 15
multipart_max_chunks = 10000
preserve_attrs = True
progress_meter = True
proxy_host =
proxy_port = 0
put_continue = False
recursive = False
recv_chunk = 65536
reduced_redundancy = False
requester_pays = False
restore_days = 1
restore_priority = Standard
secret_key = ${secret_key}
send_chunk = 65536
server_side_encryption = False
signature_v2 = False
signurl_use_https = False
simpledb_host = sdb.amazonaws.com
skip_existing = False
socket_timeout = 300
stats = False
stop_on_error = False
storage_class =
urlencoding_mode = normal
use_http_expect = False
use_https = True
use_mime_magic = True
verbosity = WARNING
website_endpoint = http://%(bucket)s.s3-website-%(location)s.amazonaws.com/
website_error =
website_index = index.html
EOM
}

create_doks_s3() {
  if [[ -z ${input_config} ]]; then
    echo "Please go through https://docs.digitalocean.com/products/spaces/resources/s3cmd/ to know about options"
    echo "for creation of bucket, please provide input"
    read -r -p "Access_key: " access_key
    read -r -p "Secret_key: " secret_key
    read -r -p "Host Base (default - nyc3.digitaloceanspaces.com): " host_base
    read -r -p "Host Bucket (default - %(bucket)s.nyc3.digitaloceanspaces.com): " host_bucket
    read -r -p "gpg_passphrase (default - trilio): " gpg_passphrase
    read -r -p "Bucket Name: " bucket_name
    read -r -p "Target Name: " target_name
    read -r -p "Target Namespace: " target_namespace
    read -r -p "thresholdCapacity (Units can be[Mi/Gi/Ti]) (default - 1000Gi): " thresholdCapacity
    read -r -p "Proceed even if resource exists y/n (default - y): " if_resource_exists_still_proceed
  fi
  if [[ -z "$if_resource_exists_still_proceed" ]]; then
    if_resource_exists_still_proceed='y'
  fi
  if [[ -z "$target_namespace" ]]; then
    target_namespace="default"
  fi
  if [[ $(kubectl get target "$target_name" -n "$target_namespace" 2>> >(logit)) ]]; then
    if kubectl get target "$target_name" -n "$target_namespace" -o 'jsonpath={.status.status}' 2>/dev/null | grep -q Unavailable; then
      echo "Target with same name already exists but is in Unavailable state"
      return 1
    else
      echo "Target with same name already exists"
      return 0
    fi
    if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
      exit 1
    else
      return 0
    fi
  fi
  if [[ -z "$gpg_passphrase" ]]; then
    gpg_passphrase="trilio"
  fi
  if [[ -z "$thresholdCapacity" ]]; then
    thresholdCapacity='1000Gi'
  fi
  if [[ -z "$host_base" ]]; then
    host_base="nyc3.digitaloceanspaces.com"
  fi
  if [[ -z "$host_bucket" ]]; then
    host_bucket="%(bucket)s.nyc3.digitaloceanspaces.com"
  fi
  call_s3cfg_doks "$access_key" "$secret_key" "$host_base" "$host_bucket" "$gpg_passphrase"
  region="$(cut -d '.' -f 1 <<<"$host_base")"
  #create bucket
  ret_val=$(s3cmd --config s3cfg_config mb s3://"$bucket_name" 2>> >(logit))
  ret_mgs=$?
  ret_val_error=$(s3cmd --config s3cfg_config mb s3://"$bucket_name" 2>&1)
  if [[ $ret_mgs -ne 0 ]]; then
    ret_code=$(echo "$ret_val" | grep 'Bucket already exists')
    ret_code_err=$(echo "$ret_val_error" | grep 'Bucket already exists')
    if [[ "$ret_code" ]] || [[ $ret_code_err ]]; then
      echo "WARNING: Bucket already exists"
    else
      echo "Error in creating spaces,please check credentials"
    fi
  fi
  #create S3 target
  url="https://$host_base"
  cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: Target
metadata:
  name: ${target_name}
  namespace: ${target_namespace}
spec:
  type: ObjectStore
  vendor: Other
  objectStoreCredentials:
    url: "$url"
    accessKey: "$access_key"
    secretKey: "$secret_key"
    bucketName: "$bucket_name"
    region: "$region"
  thresholdCapacity: $thresholdCapacity
EOF
  retcode=$?
  if [ "$retcode" -ne 0 ]; then
    echo "Target creation failed"
    return 1
  fi
}

call_s3cfg_aws() {
  access_key=$1
  secret_key=$2
  host_base=$3
  host_bucket=$4
  bucket_location=$5
  cat >s3cfg_config <<-EOM
[default]
access_key = ${access_key}
access_token =
add_encoding_exts =
add_headers =
bucket_location = ${bucket_location}
cache_file =
cloudfront_host = cloudfront.amazonaws.com
default_mime_type = binary/octet-stream
delay_updates = False
delete_after = False
delete_after_fetch = False
delete_removed = False
dry_run = False
enable_multipart = True
encoding = UTF-8
encrypt = False
expiry_date =
expiry_days =
expiry_prefix =
follow_symlinks = False
force = False
get_continue = False
gpg_command = /usr/bin/gpg
gpg_decrypt = %(gpg_command)s -d --verbose --no-use-agent --batch --yes --passphrase-fd %(passphrase_fd)s -o %(output_file)s %(input_file)s
gpg_encrypt = %(gpg_command)s -c --verbose --no-use-agent --batch --yes --passphrase-fd %(passphrase_fd)s -o %(output_file)s %(input_file)s
gpg_passphrase =
guess_mime_type = True
host_base = ${host_base}
host_bucket = ${host_bucket}
human_readable_sizes = False
ignore_failed_copy = False
invalidate_default_index_on_cf = False
invalidate_default_index_root_on_cf = True
invalidate_on_cf = False
list_md5 = False
log_target_prefix =
max_delete = -1
mime_type =
multipart_chunk_size_mb = 15
preserve_attrs = True
progress_meter = True
proxy_host =
proxy_port = 0
put_continue = False
recursive = False
recv_chunk = 4096
reduced_redundancy = False
restore_days = 1
secret_key = ${secret_key}
send_chunk = 4096
server_side_encryption = False
simpledb_host = sdb.amazonaws.com
skip_existing = False
socket_timeout = 300
urlencoding_mode = normal
use_https = True
use_mime_magic = True
verbosity = WARNING
website_endpoint = http://%(bucket)s.s3-website-% (location)s.amazonaws.com/
website_error =
website_index = index.html
EOM
}

#Function to create Aws s3 target
create_aws_s3() {
  if [[ -z ${input_config} ]]; then
    echo "Please go through https://linux.die.net/man/1/s3cmd to know about options"
    echo "for creation of bucket, please provide input"
    read -r -p "Access_key: " access_key
    read -r -p "Secret_key: " secret_key
    read -r -p "Host Base (default - s3.amazonaws.com): " host_base
    read -r -p "Host Bucket (default - %(bucket)s.s3.amazonaws.com): " host_bucket
    read -r -p "Bucket Location Region to create bucket in. As of now the regions are:
                        us-east-1, us-west-1, us-west-2, eu-west-1, eu-
                        central-1, ap-northeast-1, ap-southeast-1, ap-
                        southeast-2, sa-east-1 (default - us-east-1): " bucket_location
    read -r -p "Bucket Name: " bucket_name
    read -r -p "Target Name: " target_name
    read -r -p "Target Namespace: " target_namespace
    read -r -p "thresholdCapacity (Units can be[Mi/Gi/Ti]) (default - 1000Gi): " thresholdCapacity
    read -r -p "Proceed even if resource exists y/n (default - y): " if_resource_exists_still_proceed
  fi
  if [[ -z "$if_resource_exists_still_proceed" ]]; then
    if_resource_exists_still_proceed='y'
  fi
  if [[ -z "$target_namespace" ]]; then
    target_namespace="default"
  fi
  if [[ $(kubectl get target "$target_name" -n "$target_namespace" 2>> >(logit)) ]]; then
    if kubectl get target "$target_name" -n "$target_namespace" -o 'jsonpath={.status.status}' 2>/dev/null | grep -q Unavailable; then
      echo "Target with same name already exists but is in Unavailable state"
      return 1
    else
      echo "Target with same name already exists"
      return 0
    fi

    if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
      exit 1
    else
      return 0
    fi
  fi
  if [[ -z "$thresholdCapacity" ]]; then
    thresholdCapacity='1000Gi'
  fi
  if [[ -z "$host_base" ]]; then
    host_base="s3.amazonaws.com"
  fi
  if [[ -z "$host_bucket" ]]; then
    host_bucket="%(bucket)s.s3.amazonaws.com"
  fi
  if [[ -z "$bucket_location" ]]; then
    bucket_location="us-east-1"
  fi
  call_s3cfg_aws "$access_key" "$secret_key" "$host_base" "$host_bucket" "$bucket_location"
  #create bucket
  ret_val=$(s3cmd --config s3cfg_config mb s3://"$bucket_name" 2>&1)
  ret_mgs=$?
  if [[ "$ret_mgs" -ne 0 ]]; then
    ret_code=$(echo "$ret_val" | grep 'BucketAlreadyOwnedByYou')
    if [[ "$ret_code" ]]; then
      echo "WARNING: Bucket already exists"
    else
      echo "Error in creating bucket,please check credentials"
      return 1
    fi
  fi
  #create S3 target
  region=$(s3cmd --config s3cfg_config info s3://"$bucket_name"/ | grep Location | cut -d':' -f2- | sed 's/^ *//g')
  url="https://s3.amazonaws.com"
  cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: Target
metadata:
  name: ${target_name}
  namespace: ${target_namespace}
spec:
  type: ObjectStore
  vendor: Other
  objectStoreCredentials:
    url: "$url"
    accessKey: "$access_key"
    secretKey: "$secret_key"
    bucketName: "$bucket_name"
    region: "$region"
  thresholdCapacity: $thresholdCapacity
EOF
  retcode=$?
  if [ "$retcode" -ne 0 ]; then
    echo "Target creation failed"
    return 1
  fi
}

#This module is used to create target to be used for TVK backup and restore
create_target() {
  if [[ -z ${input_config} ]]; then
    echo -e "Target can be created on NFS or s3 compatible storage\n1.NFS (default) \n2.S3"
    read -r -p "select option: " target_type
  else
    if [[ $target_type == 'NFS' ]]; then
      target_type=1
    elif [[ $target_type == 'S3' ]]; then
      target_type=2
    else
      echo "Wrong value provided for target"
    fi
  fi
  if [[ -z "$target_type" ]]; then
    target_type=1
  fi
  case $target_type in
  2)
    ret=$(s3cmd --version 2>/dev/null)
    if [[ -z $ret ]]; then
      echo "This functionality requires s3cmd utility installed"
      echo "Please check README or follow https://s3tools.org/s3cmd"
      return 1
    fi
    if [[ -z ${input_config} ]]; then
      echo -e "Please select vendor\n1.Digital_Ocean\n2.Amazon_AWS"
      read -r -p "select option: " vendor_type
    else
      if [[ $vendor_type == "Digital_Ocean" ]]; then
        vendor_type=1
      elif [[ $vendor_type == "Amazon_AWS" ]]; then
        vendor_type=2
      else
        echo "Wrong value provided for target"
      fi
    fi
    case $vendor_type in
    1)
      create_doks_s3
      ret_code=$?
      if [ "$ret_code" -ne 0 ]; then
        return 1
      fi
      ;;
    2)
      create_aws_s3
      ret_code=$?
      if [ "$ret_code" -ne 0 ]; then
        return 1
      fi
      ;;
    *)
      echo "Wrong selection"
      return 1
      ;;
    esac
    shift
    ;;
  1)
    if [[ -z ${input_config} ]]; then
      read -r -p "Target Name: " target_name
      read -r -p "NFSserver: " nfs_server
      read -r -p "namespace: " target_namespace
      read -r -p "Export Path: " nfs_path
      read -r -p "NFSoption (default - nfsvers=4): " nfs_options
      read -r -p "thresholdCapacity (Units can be[Mi/Gi/Ti]) (default - 1000Gi): " thresholdCapacity
      read -r -p "Proceed even if resource exists y/n (default - y): " if_resource_exists_still_proceed
    fi
    if [[ -z "$if_resource_exists_still_proceed" ]]; then
      if_resource_exists_still_proceed='y'
    fi
    if [[ $(kubectl get target "$target_name" -n "$target_namespace" 2>/dev/null) ]]; then
      if kubectl get target "$target_name" -n "$target_namespace" -o 'jsonpath={.status.status}' 2>/dev/null | grep -q Unavailable; then
        echo "Target with same name already exists but is in Unavailable state"
        return 1
      else
        echo "Target with same name already exists"
        return 0
      fi
      if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
        exit 1
      else
        return 0
      fi
    fi
    if [[ -z "$thresholdCapacity" ]]; then
      thresholdCapacity='1000Gi'
    fi
    if [[ -z "$nfs_options" ]]; then
      nfs_options='nfsvers=4'
    fi
    echo "Creating target..."
    cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: Target
metadata:
  name: ${target_name}
  namespace: ${target_namespace}
spec:
  type: NFS
  vendor: Other
  nfsCredentials:
    nfsExport: ${nfs_server}:${nfs_path}
    nfsOptions: ${nfs_options}
  thresholdCapacity: ${thresholdCapacity}
EOF
    retcode=$?
    if [ "$retcode" -ne 0 ]; then
      echo "Target creation failed"
      return 1
    fi
    ;;
  *)
    echo "Wrong selection"
    return 1
    ;;
  esac
  shift
  cmd="kubectl get target $target_name -n $target_namespace -o 'jsonpath={.status.status}' 2>/dev/null | grep -e Available -e Unavailable"
  wait_install 20 "$cmd"
  if ! kubectl get target "$target_name" -n "$target_namespace" -o 'jsonpath={.status.status}' 2>/dev/null | grep -q Available; then
    echo "Failed to create target"
    return 1
  else
    echo "Target is Available to use"
  fi
}

#This module is used to test TVK backup and restore for user.
sample_test() {
  if [[ -z ${input_config} ]]; then
    echo "Please provide input for test demo"
    read -r -p "Target Name: " target_name
    read -r -p "Target Namespace: " target_namespace
    read -r -p "Backupplan name (default - trilio-test-backup): " bk_plan_name
    read -r -p "Backup Name (default - trilio-test-backup): " backup_name
    read -r -p "Backup Namespace Name (default - trilio-test-backup): " backup_namespace
    read -r -p "Proceed even if resource exists y/n (default - y): " if_resource_exists_still_proceed
  fi
  if [[ -z "$if_resource_exists_still_proceed" ]]; then
    if_resource_exists_still_proceed='y'
  fi
  if [[ -z "$backup_namespace" ]]; then
    backup_namespace="trilio-test-backup"
  fi
  if [[ -z "$backup_name" ]]; then
    backup_name="trilio-test-backup"
  fi
  if [[ -z "$bk_plan_name" ]]; then
    bk_plan_name="trilio-test-backup"
  fi
  res=$(kubectl get ns $backup_namespace 2>/dev/null)
  if [[ -z "$res" ]]; then
    kubectl create ns $backup_namespace 2>/dev/null
  fi
  storage_class=$(kubectl get storageclass | grep -w '(default)' | awk '{print $1}')
  if [[ -z "$storage_class" ]]; then
    echo "No default storage class found, need one to proceed"
    return 1
  fi
  #Add stable helm repo
  helm repo add stable https://charts.helm.sh/stable 1>> >(logit) 2>> >(logit)
  helm repo update 1>> >(logit) 2>> >(logit)
  if [[ -z ${input_config} ]]; then
    echo -e "Select an the backup way\n1.Label based(MySQL)\n2.Namespace based(Wordpress)\n3.Operator based(Mysql Operator)\n4.Helm based(Mongodb)"
    read -r -p "Select option: " backup_way
  else
    if [[ $backup_way == "Label_based" ]]; then
      backup_way=1
    elif [[ $backup_way == "Namespace_based" ]]; then
      backup_way=2
    elif [[ $backup_way == "Operator_based" ]]; then
      backup_way=3
    elif [[ $backup_way == "Helm_based" ]]; then
      backup_way=4
    else
      echo "Backup way is wrong/not defined"
      return 1
    fi
  fi
  if [[ -z $backup_way ]]; then
    echo "Please provide valid backup-way..exiting.."
    return 1
  fi

  #Check if yq is installed
  ret=$(yq -V 2>/dev/null)
  if [[ -z $ret ]]; then
    echo "This functionality requires yq utility installed"
    echo "Please check README or follow https://github.com/mikefarah/yq"
    return 1
  fi
  #Create backupplan template
  cat >backupplan.yaml <<-EOM
apiVersion: triliovault.trilio.io/v1
kind: BackupPlan
metadata:
  name: trilio-test-label
  namespace: trilio-test-backup
spec:
  backupNamespace: trilio-test-backup
  backupConfig:
    target:
      name: 
      namespace: 
    schedulePolicy:
      incrementalCron:
        schedule: "* 0 * * *"
    retentionPolicy:
      name: sample-policy
      namespace: default
EOM
  ret_code=$?
  if [ "$ret_code" -ne 0 ]; then
    echo "Cannot write backupplan.yaml file, please check file system permissions"
    return 1
  fi
  case $backup_way in
  1)
    ## Install mysql helm chart
    #check if app is already installed with same name
    if helm list -n "$backup_namespace" | grep -w -q mysql-qa; then
      echo "Application exists"
      if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
        exit 1
      fi
      echo "Waiting for application to be in Ready state"
      cmd="kubectl get pods -l app=mysql-qa -n $backup_namespace 2>/dev/null | grep Running"
      wait_install 15 "$cmd"
      if ! kubectl get pods -l app=mysql-qa -n $backup_namespace 2>/dev/null | grep -q Running; then
        echo "Application taking more time than usual to be in Ready state, Exiting.."
        exit 1
      fi
    else
      helm install mysql-qa stable/mysql -n $backup_namespace 1>> >(logit) 2>> >(logit)
      echo "Installing Application"
      cmd="kubectl get pods -l app=mysql-qa -n $backup_namespace 2>/dev/null | grep Running"
      wait_install 15 "$cmd"
      if ! kubectl get pods -l app=mysql-qa -n $backup_namespace 2>/dev/null | grep -q Running; then
        echo "Application installation failed"
        return 1
      fi
      echo "Requested application is installed successfully"
    fi
    yq eval -i 'del(.spec.backupPlanComponents)' backupplan.yaml 1>> >(logit) 2>> >(logit)
    yq eval -i '.spec.backupPlanComponents.custom[0].matchLabels.app="mysql-qa"' backupplan.yaml 1>> >(logit) 2>> >(logit)
    ;;
  2)
    if helm list -n $backup_namespace | grep -w -q my-wordpress 2>> >(logit); then
      echo "Application exists"
      if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
        exit 1
      fi
      echo "Waiting for application to be in Ready state"
    else
      #Add bitnami helm repo
      helm repo add bitnami https://charts.bitnami.com/bitnami 1>> >(logit) 2>> >(logit)
      helm install my-wordpress bitnami/wordpress -n $backup_namespace 1>> >(logit) 2>> >(logit)
      echo "Installing Application"
    fi
    runtime=20
    spin='-\|/'
    i=0
    endtime=$(python3 -c "import time;timeout = int(time.time()) + 60*$runtime;print(\"{0}\".format(timeout))")
    while [[ $(python3 -c "import time;timeout = int(time.time());print(\"{0}\".format(timeout))") -le $endtime ]] && kubectl get pod -l app.kubernetes.io/instance=my-wordpress -n $backup_namespace -o jsonpath="{.items[*].status.conditions[*].status}" | grep -q False; do
      i=$(((i + 1) % 4))
      printf "\r %s" "${spin:$i:1}"
      sleep .1
    done
    if kubectl get pod -l app.kubernetes.io/instance=my-wordpress -n $backup_namespace -o jsonpath="{.items[*].status.conditions[*].status}" | grep -q False; then
      echo "Wordpress Application taking more time than usual to be in Ready state, Exiting.."
      return 1
    fi
    echo "Requested application is Up and Running!"
    yq eval -i 'del(.spec.backupPlanComponents)' backupplan.yaml 1>> >(logit) 2>> >(logit)
    ;;
  3)
    if helm list -n $backup_namespace | grep -w -q mysql-operator 2>> >(logit); then
      echo "Application exists"
      if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
        exit 1
      fi
      echo "Waiting for application to be in Ready state"
    else
      echo "MySQL operator will require enough resources, else the deployment will fail"
      helm repo add presslabs https://presslabs.github.io/charts 1>> >(logit) 2>> >(logit)
      errormessage=$(helm install mysql-operator presslabs/mysql-operator -n $backup_namespace 2>> >(logit))
      if echo "$errormessage" | grep -Eq 'Error:|error:'; then
        echo "Mysql operator Installation failed with error: $errormessage"
        return 1
      fi
      echo "Installing MySQL Operator..."
    fi
    runtime=15
    spin='-\|/'
    i=0
    endtime=$(python3 -c "import time;timeout = int(time.time()) + 60*$runtime;print(\"{0}\".format(timeout))")
    while [[ $(python3 -c "import time;timeout = int(time.time());print(\"{0}\".format(timeout))") -le $endtime ]] && kubectl get pod -l app=mysql-operator -n $backup_namespace -o jsonpath="{.items[*].status.conditions[*].status}" | grep -q False; do
      i=$(((i + 1) % 4))
      printf "\r %s" "${spin:$i:1}"
      sleep .1
    done
    if kubectl get pod -l app=mysql-operator -n $backup_namespace -o jsonpath="{.items[*].status.conditions[*].status}" | grep -q False; then
      echo "MySQL operator taking more time than usual to be in Ready state, Exiting.."
      return 1
    fi
    if ! kubectl get pods -l mysql.presslabs.org/cluster=my-cluster -n $backup_namespace --ignore-not-found 1>> >(logit); then
      echo "Mysql cluster already exists.."
      echo "Waiting for application to be in Ready state"
    else
      #Create a MySQL cluster
      kubectl apply -f https://raw.githubusercontent.com/bitpoke/mysql-operator/master/examples/example-cluster-secret.yaml -n $backup_namespace 2>> >(logit)
      kubectl apply -f https://raw.githubusercontent.com/bitpoke/mysql-operator/master/examples/example-cluster.yaml -n $backup_namespace 2>> >(logit)
      echo "Installing MySQL cluster..."
      sleep 10
    fi
    runtime=15
    spin='-\|/'
    i=0
    endtime=$(python3 -c "import time;timeout = int(time.time()) + 60*$runtime;print(\"{0}\".format(timeout))")
    while [[ $(python3 -c "import time;timeout = int(time.time());print(\"{0}\".format(timeout))") -le $endtime ]] && kubectl get pods -l mysql.presslabs.org/cluster=my-cluster -n $backup_namespace -o jsonpath="{.items[*].status.conditions[*].status}" | grep -q False; do
      i=$(((i + 1) % 4))
      printf "\r %s" "${spin:$i:1}"
      sleep .1
    done
    sleep 5
    while [[ $(python3 -c "import time;timeout = int(time.time());print(\"{0}\".format(timeout))") -le $endtime ]] && kubectl get pods -l mysql.presslabs.org/cluster=my-cluster -n $backup_namespace -o jsonpath="{.items[*].status.conditions[*].status}" | grep -q False; do
      i=$(((i + 1) % 4))
      printf "\r %s" "${spin:$i:1}"
      sleep .1
    done
    if kubectl get pods -l mysql.presslabs.org/cluster=my-cluster -n $backup_namespace -o jsonpath="{.items[*].status.conditions[*].status}" | grep -q False; then
      echo "MySQL cluster taking more time than usual to be in Ready state, Exiting.."
      return 1
    fi
    echo "Requested application is Up and Running!"
    #Creating backupplan
    {
      yq eval -i 'del(.spec.backupPlanComponents)' backupplan.yaml
      yq eval -i '.spec.backupPlanComponents.operators[0].operatorId="my-cluster"' backupplan.yaml
      yq eval -i '.spec.backupPlanComponents.operators[0].customResources[0].groupVersionKind.group="mysql.presslabs.org" | .spec.backupPlanComponents.operators[0].customResources[0].groupVersionKind.group style="double"' backupplan.yaml
      yq eval -i '.spec.backupPlanComponents.operators[0].customResources[0].groupVersionKind.version="v1alpha1" | .spec.backupPlanComponents.operators[0].customResources[0].groupVersionKind.version style="double"' backupplan.yaml
      yq eval -i '.spec.backupPlanComponents.operators[0].customResources[0].groupVersionKind.kind="MysqlCluster" | .spec.backupPlanComponents.operators[0].customResources[0].groupVersionKind.kind style="double"' backupplan.yaml
      yq eval -i '.spec.backupPlanComponents.operators[0].customResources[0].objects[0]="my-cluster"' backupplan.yaml
      yq eval -i '.spec.backupPlanComponents.operators[0].operatorResourceSelector[0].matchLabels.name="mysql-operator"' backupplan.yaml
      yq eval -i '.spec.backupPlanComponents.operators[0].applicationResourceSelector[0].matchLabels.app="mysql-operator"' backupplan.yaml
    } 1>> >(logit) 2>> >(logit)
    ;;
  4)
    if helm list -n $backup_namespace | grep -q -w mongotest; then
      echo "Application exists"
      if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
        exit 1
      fi
      echo "Waiting for application to be in Ready state"
    else
      {
        helm repo add bitnami https://charts.bitnami.com/bitnami
        helm repo update 1>> >(logit)
        helm install mongotest bitnami/mongodb -n $backup_namespace
      } 2>> >(logit)
      echo "Installing App..."
    fi
    runtime=15
    spin='-\|/'
    i=0
    endtime=$(python3 -c "import time;timeout = int(time.time()) + 60*$runtime;print(\"{0}\".format(timeout))")
    while [[ $(python3 -c "import time;timeout = int(time.time());print(\"{0}\".format(timeout))") -le $endtime ]] && kubectl get pod -l app.kubernetes.io/name=mongodb -n $backup_namespace -o jsonpath="{.items[*].status.conditions[*].status}" | grep -q -w False; do
      i=$(((i + 1) % 4))
      printf "\r %s" "${spin:$i:1}"
      sleep .1
    done
    if kubectl get pod -l app.kubernetes.io/name=mongodb -n $backup_namespace -o jsonpath="{.items[*].status.conditions[*].status}" | grep -q False; then
      echo "Mongodb Application taking more time than usual to be in Ready state, Exiting.."
      return 1
    fi
    echo "Requested application is Up and Running!"
    yq eval -i 'del(.spec.backupPlanComponents)' backupplan.yaml 1>> >(logit) 2>> >(logit)
    yq eval -i '.spec.backupPlanComponents.helmReleases[0]="mongotest"' backupplan.yaml 1>> >(logit) 2>> >(logit)
    ;;
  *)
    echo "Wrong choice"
    ;;
  esac
  #check if backupplan with same name already exists
  if [[ $(kubectl get backupplan $bk_plan_name -n $backup_namespace 2>/dev/null) ]]; then
    echo "Backupplan with same name already exists"
    if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
      exit 1
    fi
    echo "Waiting for Backupplan to be in Available state"
  else
    #Applying backupplan manifest
    {
      yq eval -i '.metadata.name="'$bk_plan_name'"' backupplan.yaml
      yq eval -i '.metadata.namespace="'$backup_namespace'"' backupplan.yaml
      yq eval -i '.spec.backupNamespace="'$backup_namespace'"' backupplan.yaml
      yq eval -i '.spec.backupConfig.target.name="'"$target_name"'"' backupplan.yaml
      yq eval -i '.spec.backupConfig.target.namespace="'"$target_namespace"'"' backupplan.yaml
    } 1>> >(logit) 2>> >(logit)
    echo "Creating backupplan..."
    cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: Policy
metadata:
  name: sample-policy
spec:
  type: Retention
  default: false
  retentionConfig:
    latest: 30
    weekly: 7
    monthly: 30
EOF
    retcode=$?
    if [ "$retcode" -ne 0 ]; then
      echo "Erro while applying policy"
      return 1
    fi
    if ! kubectl apply -f backupplan.yaml -n $backup_namespace; then
      echo "Backupplan creation failed"
      return 1
    fi
  fi
  cmd="kubectl get backupplan $bk_plan_name -n $backup_namespace -o 'jsonpath={.status.status}' 2>/dev/null | grep -e Available -e Unavailable"
  wait_install 10 "$cmd"
  if ! kubectl get backupplan $bk_plan_name -n $backup_namespace -o 'jsonpath={.status.status}' 2>/dev/null | grep -q Available; then
    echo "Backupplan is in Unavailable state"
    return 1
  else
    echo "Backupplan is in Available state"
  fi
  rm -f backupplan.yaml
  if [[ $(kubectl get backup $backup_name -n $backup_namespace 2>> >(logit)) ]]; then
    echo "Backup with same name already exists"
    if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
      exit 1
    fi
    echo "Waiting for Backup to be in Available state"
  else
    echo "Creating Backup..."
    #Applying backup manifest
    cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: Backup
metadata:
  name: ${backup_name}
  namespace: ${backup_namespace}
spec:
  type: Full
  backupPlan:
    name: ${bk_plan_name}
    namespace: ${backup_namespace}
EOF
    retcode=$?
    if [ "$retcode" -ne 0 ]; then
      echo "Error while creating backup"
      return 1
    fi
  fi
  cmd="kubectl get backup $backup_name -n $backup_namespace -o 'jsonpath={.status.status}' 2>/dev/null | grep -e Available -e Failed"
  wait_install 60 "$cmd"
  if ! kubectl get backup $backup_name -n $backup_namespace -o 'jsonpath={.status.status}' 2>/dev/null | grep -wq Available; then
    echo "Backup Failed"
    return 1
  else
    echo "Backup is Available Now"
  fi
  if [[ -z ${input_config} ]]; then
    read -r -p "whether restore test should also be done? y/n: " restore
  fi
  if [[ ${restore} == "Y" ]] || [[ ${restore} == "y" ]] || [[ ${restore} == "True" ]]; then
    if [[ -z ${input_config} ]]; then
      read -r -p "Restore Namepsace (default - trilio-test-rest): " restore_namespace
      read -r -p "Restore name (default - trilio-test-restore): " restore_name
    fi
    if [[ -z "$restore_namespace" ]]; then
      restore_namespace="trilio-test-rest"
    fi
    if ! kubectl get ns "$restore_namespace" 1>> >(logit) 2>> >(logit); then
      echo "Namespace with name $restore_namespace already Exists"
      if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
        exit 1
      fi
      if ! kubectl create ns "$restore_namespace" 1>> >(logit) 2>> >(logit); then
        echo "Error while creating $restore_namespace namespace"
        return 1
      fi
    fi
    if [[ -z "$restore_name" ]]; then
      restore_name="trilio-test-restore"
    fi
    if [[ $(kubectl get restore $restore_name -n $restore_namespace 2>/dev/null) ]]; then
      echo "Restore with same name already exists"
      if [[ "$if_resource_exists_still_proceed" != "Y" ]] && [[ "$if_resource_exists_still_proceed" != "y" ]]; then
        exit 1
      fi
      echo "Waiting for Restore to be in Available state"
    else
      echo "Creating restore..."
      #Applying restore manifest
      cat <<EOF | kubectl apply -f - 1>> >(logit) 2>> >(logit)
apiVersion: triliovault.trilio.io/v1
kind: Restore
metadata:
  name: ${restore_name}
  namespace: ${restore_namespace}
spec:
  source:
    type: Backup
    backup:
      namespace: ${backup_namespace}
      name: ${backup_name}
    target:
      name: ${target_name}
      namespace: ${target_namespace}
  restoreNamespace: ${restore_namespace}
  skipIfAlreadyExists: true
EOF
      retcode=$?
      if [ "$retcode" -ne 0 ]; then
        echo "Error while restoring"
        return 1
      fi
    fi
    cmd="kubectl get restore $restore_name -n $restore_namespace -o 'jsonpath={.status.status}' 2>/dev/null | grep -e Completed -e Failed"
    wait_install 60 "$cmd"
    if ! kubectl get restore $restore_name -n $restore_namespace -o 'jsonpath={.status.status}' 2>/dev/null | grep -wq 'Completed'; then
      echo "Restore Failed"
      return 1
    else
      echo "Restore is Completed"
    fi
  fi
}

print_usage() {
  echo "
--------------------------------------------------------------
tvk-oneclick - Installs, Configures UI, Create sample backup/restore test
Usage:
kubectl tvk-oneclick [options] [arguments]
Options:
        -h, --help                show brief help
        -n, --noninteractive      run script in non-interactive mode.for this you need to provide config file
        -i, --install_tvk         Installs TVK and it's free trial license.
        -c, --configure_ui        Configures TVK UI
        -t, --target              Created Target for backup and restore jobs
        -s, --sample_test         Create sample backup and restore jobs
	-p, --preflight           Checks if all the pre-requisites are satisfied
-----------------------------------------------------------------------
"
}

main() {
  for i in "$@"; do
    #key="$1"
    case $i in
    -h | --help)
      print_usage
      exit 0
      ;;
    -n | --noninteractive)
      export Non_interact=True
      echo "Flag set to run cleanup in non-interactive mode"
      echo
      ;;
    -i | --install_tvk)
      export TVK_INSTALL=True
      #echo "Flag set to install TVK product"
      shift
      echo
      ;;
    -c | --configure_ui)
      export CONFIGURE_UI=True
      #echo "flag set to configure ui"
      echo
      ;;
    -t | --target)
      export TARGET=True
      #echo "flag set to create backup target"
      shift
      echo
      ;;
    -s | --sample_test)
      export SAMPLE_TEST=True
      #echo "flag set to test sample  backup and restore of application "
      echo
      ;;
    -p | --preflight)
      export PREFLIGHT=True
      echo
      ;;
    *)
      echo "Incorrect option, check usage below..."
      echo
      print_usage
      exit 1
      ;;
    esac
    shift
  done
  export input_config=""
  if [ ${Non_interact} ]; then
    read -r -p "Please enter path for config file: " input_config
    # shellcheck source=/dev/null
    # shellcheck disable=SC2086
    . $input_config
    export input_config=$input_config
  fi
  if [[ ${PREFLIGHT} == 'True' ]]; then
    preflight_checks
  fi
  if [[ ${TVK_INSTALL} == 'True' ]]; then
    install_tvk
  fi
  if [[ ${CONFIGURE_UI} == 'True' ]]; then
    configure_ui
  fi
  if [[ ${TARGET} == 'True' ]]; then
    create_target
  fi
  if [[ ${SAMPLE_TEST} == 'True' ]]; then
    sample_test
  fi

}

logit() {
  # shellcheck disable=SC2162
  while read; do
    time=$(python3 -c "import datetime;e = datetime.datetime.now();print(\"%s\" % e)")
    echo "$time $REPLY" >>"${LOG_FILE}"
  done
}

LOG_FILE="/tmp/tvk_oneclick_stderr"

ret=$(python3 --version 2>/dev/null)
if [[ -z $ret ]]; then
  echo "Plugin requires python3 installed"
  echo "Please install and check"
  return 1
fi
ret=$(pip3 --version 2>> >(logit))
ret_code=$?
if [ "$ret_code" -ne 0 ]; then
  echo "This plugin requires pip3 to be installed.Please follow README"
  exit 1
fi
ret=$(pip3 install packaging 1>> >(logit) 2>> >(logit))
ret_code=$?
if [ "$ret_code" -ne 0 ]; then
  echo "pip3 install is failing.Please check the permisson and try again.."
  exit 1
fi
main "$@"
