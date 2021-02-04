#!/usr/bin/env python3
import os
import shutil
import logging
import argparse
import sys
from collections import defaultdict
from datetime import datetime

import kubernetes.client
import kubernetes.client.rest
import kubernetes.config
import yaml
import zipfile

log = logging.getLogger(__name__)

K8S = "K8s_"

OPERATOR_GROUP = 'operators.coreos.com'
APIEXTENSIONS_GROUP = 'apiextensions.k8s.io'
SNAPSHOT_STORAGE_GROUP = 'snapshot.storage.k8s.io'
ADMISSIONREGISTRATION_GROUP = 'admissionregistration.k8s.io/v1beta1'
TRILIOVAULT_GROUP = 'triliovault.trilio.io'
CSI_STORAGE_GROUP = 'csi.storage.k8s.io'

STORAGE_GV = 'storage.k8s.io/v1'
CORE_GV = 'v1'
BATCH_GV = 'batch/v1'

NAMESPACES = 'namespaces'
EVENTS = 'events'
PODS = 'pods'
JOBS = 'jobs'
CRD = 'customresourcedefinitions'
STORAGE_CLASS = 'storageclasses'
VOLUME_ATTACHMENT = 'volumeattachments'
VOLUME_SNAPSHOT = 'volumesnapshots'
VOLUME_SNAPSHOT_CLASS = 'volumesnapshotclasses'
CLUSTER_SERVICE_VERSION = 'clusterserviceversions'

def main():
    if not sys.version_info.major == 3 and sys.version_info.minor >= 6:
        print("You are using Python {}.{}.".format(sys.version_info.major, sys.version_info.minor))
        raise Exception("Python 3.6 or higher is required.")

    arg_parser = argparse.ArgumentParser()
    arg_parser.add_argument('--clustered', action='store_true', help='whether clustered installtion '
                                                                     'of trilio application')
    arg_parser.add_argument('--namespaces', nargs='*', help='namespaces to look for resources')
    arg_parser.add_argument('--kube_config', help='path to the kubernetes kubeconfig', default='~/.kube/config')
    arg_parser.add_argument('--no-clean', action='store_true', help='don\'t clean output directory after zip')
    arg_parser.add_argument('--log-level', default='INFO')
    args = arg_parser.parse_args()
    if not args.clustered and not args.namespaces:
        arg_parser.error('Atleast one namespaced should be specified for namespace installation')

    kube_path = os.path.expanduser(args.kube_config)
    if not os.path.isfile(kube_path):
        arg_parser.error('Kubeconfig doesn\'t exists at specified location :{}'.format(args.kube_config))

    logging.basicConfig(format='%(levelname)s: %(message)s', level=args.log_level)

    kubernetes.config.load_kube_config(config_file=args.kube_config)

    log_collector = LogCollector('triliovault-' + datetime.now().strftime("%d-%m-%Y-%H-%M-%S"))
    log_collector.clean_output = not args.no_clean
    log_collector.namespaced = not args.clustered
    log_collector.namespaces = args.namespaces
    log_collector.dump_all()


class LogCollector:
    def __init__(self, output_dir):
        self.client = kubernetes.client.ApiClient()
        self.output_dir = output_dir
        self.clean_output = True

    def call(self, *args, **kwargs):
        kwargs.setdefault('response_type', object)
        kwargs.setdefault('auth_settings', ['BearerToken'])
        kwargs.setdefault('_return_http_data_only', True)
        return self.client.call_api(*args, **kwargs)

    def dump_all(self):
        if self.clean_output:
            shutil.rmtree(self.output_dir, ignore_errors=True)

        resource_map = dict()

        log.info("Fetching API Group version list")
        api_group_versions = self.call('/apis/', 'GET', response_type='V1APIGroupList').groups

        log.info("Checking Namespaces")
        core_gv_resources = self.get_api_gv_resources(CORE_GV)
        namespace_resource = get_resource_by_name(core_gv_resources, NAMESPACES)
        namespace_objects = self.get_resource_objects(get_api_group_version_resource_path(CORE_GV), namespace_resource)
        namespaces = get_object_names(namespace_objects)

        if self.namespaces and not set(self.namespaces).issubset(namespaces):
            log.error("Specified namespaces doesn't exists in the cluster")
            return

        log.info("Checking Cluster Service Version")
        operator_gv_list = get_gv_by_group(api_group_versions, OPERATOR_GROUP, isPreferredVersion=False)
        operator_gv_resources_map = self.get_api_gv_resources_map(operator_gv_list)
        csv_resource_map = get_resource_gv_by_name(operator_gv_resources_map, CLUSTER_SERVICE_VERSION)
        csv_objects = self.get_gv_resource_objects(csv_resource_map)
        csv_objects = filter_csv(csv_objects)
        for csv in csv_objects:
            resource_dir = os.path.join(csv['kind'])
            self.write_yaml(resource_dir, csv)

        log.info("Checking API Extension Group")
        apiext_gv = get_gv_by_group(api_group_versions, APIEXTENSIONS_GROUP)
        apiext_gv_resources = self.get_api_gv_resources(apiext_gv)
        crd_resource = get_resource_by_name(apiext_gv_resources, CRD)
        crd_objects = self.get_resource_objects(get_api_group_version_resource_path(apiext_gv), crd_resource)
        crd_objects = filter_crd(crd_objects)
        for crd in crd_objects:
            resource_dir = os.path.join(crd_resource.kind)
            self.write_yaml(resource_dir, crd)

        log.info("Checking Snapshot Storage Group")
        snap_gv = get_gv_by_group(api_group_versions, SNAPSHOT_STORAGE_GROUP)
        snap_gv_resources = self.get_api_gv_resources(snap_gv)
        volsnap_resource = get_resource_by_name(snap_gv_resources, VOLUME_SNAPSHOT)
        volsnap_objects = self.get_resource_objects(get_api_group_version_resource_path(snap_gv), volsnap_resource)
        for object in volsnap_objects:
            resource_dir = os.path.join(volsnap_resource.kind)
            self.write_yaml(resource_dir, object)

        volsnapclass_resource = get_resource_by_name(snap_gv_resources, VOLUME_SNAPSHOT_CLASS)
        volsnapclass_objects = self.get_resource_objects(get_api_group_version_resource_path(snap_gv),
                                                         volsnapclass_resource)
        for object in volsnapclass_objects:
            resource_dir = os.path.join(volsnapclass_resource.kind)
            self.write_yaml(resource_dir, object)

        log.info("Checking Admission Registration Group")
        admission_gv = get_gv_by_group(api_group_versions, ADMISSIONREGISTRATION_GROUP)
        admission_gv_resources = self.get_api_gv_resources(admission_gv)
        for resource in admission_gv_resources:
            object_list = self.get_resource_objects(get_api_group_version_resource_path(admission_gv), resource)
            resource_dir = os.path.join(resource.kind)
            for object in object_list:
                if object['metadata']['name'].startswith('k8s-triliovault'):
                    self.write_yaml(resource_dir, object)

        log.info("Checking Storage Group")
        storage_gv_resources = self.get_api_gv_resources(STORAGE_GV)
        sc_resource = get_resource_by_name(storage_gv_resources, STORAGE_CLASS)
        sc_objects = self.get_resource_objects(get_api_group_version_resource_path(STORAGE_GV), sc_resource)
        for sc in sc_objects:
            resource_dir = os.path.join(sc_resource.kind)
            self.write_yaml(resource_dir, sc)

        log.info("Checking Trilio Group")
        trilio_gv = get_gv_by_group(api_group_versions, TRILIOVAULT_GROUP)
        trilio_gv_resources = self.get_api_gv_resources(trilio_gv)
        for resource in trilio_gv_resources:
            object_list = self.get_resource_objects(get_api_group_version_resource_path(trilio_gv), resource)
            resource_dir = os.path.join(resource.kind)
            for object in object_list:
                self.write_yaml(resource_dir, object)

        log.info("Checking Batch Group")
        batch_gv_resources = self.get_api_gv_resources(BATCH_GV)
        job_resource = get_resource_by_name(batch_gv_resources, JOBS)
        job_objects = self.get_resource_objects(get_api_group_version_resource_path(BATCH_GV), job_resource)
        job_objects = filter_k8s_jobs(job_objects)
        for job in job_objects:
            resource_dir = os.path.join(K8S + job_resource.kind)
            self.write_yaml(resource_dir, job)

        log.info("Checking Core Group")
        core_gv_resources = self.get_api_gv_resources(CORE_GV)
        pod_resource = get_resource_by_name(core_gv_resources, PODS)
        pod_objects = self.get_resource_objects(get_api_group_version_resource_path(CORE_GV), pod_resource)
        pod_names, pod_objects = filter_pods(pod_objects, job_objects)
        resource_map[pod_resource.kind] = pod_names

        for pod in pod_objects:
            resource_dir = os.path.join(pod_resource.kind)
            self.write_yaml(resource_dir, pod)
            self.write_logs(resource_dir, pod)

        event_resource = get_resource_by_name(core_gv_resources, EVENTS)
        event_objects = self.get_resource_objects(get_api_group_version_resource_path(CORE_GV), event_resource)
        events = aggregate_events(event_objects, resource_map)
        self.write_events(events)

        # Zip directory
        self.zipdir()

    # get_api_gv_resources returns list of resources for given group version
    def get_api_gv_resources(self, api_group_version):
        if not api_group_version:
            return []
        resource_path = get_api_group_version_resource_path(api_group_version)
        resources = self.call(resource_path, 'GET', response_type='V1APIResourceList').resources
        resources = [resource for resource in resources if 'list' in resource.verbs]
        return resources

    # get_api_gv_resources returns list of resources for given group version
    def get_api_gv_resources_map(self, api_group_version_list):
        resource_map = dict()
        for api_group in api_group_version_list:
            resource_path = get_api_group_version_resource_path(api_group)
            resources = self.call(resource_path, 'GET', response_type='V1APIResourceList').resources
            resources = [resource for resource in resources if 'list' in resource.verbs]
            resource_map[api_group] = resources
        return resource_map

    # get_gv_resource_objects returns list of objects for given resource_path
    def get_gv_resource_objects(self, gv_resource_map):
        resource_objects = []
        for gv, resource in gv_resource_map.items():
            resource_objects.extend(self.get_resource_objects(get_api_group_version_resource_path(gv), resource))
        return resource_objects

    # get_resource_objects returns list of objects for given resource_path
    def get_resource_objects(self, resource_path, resource):
        if not resource:
            return []
        if resource.namespaced and self.namespaced:
            resource_objects = []
            for namespace in self.namespaces:
                list_path = '{}/namespaces/{}/{}'.format(resource_path, namespace, resource.name)
                result = self.call(list_path, 'GET')
                resource_objects.extend(result['items'])
            return resource_objects
        else:
            list_path = '{}/{}'.format(resource_path, resource.name)
            result = self.call(list_path, 'GET')
            return result['items']

    # write_events writes events
    def write_events(self, events_details):
        for namespace, event_objects in events_details.items():
            resource_dir = os.path.join(self.output_dir, 'Events', namespace)
            os.makedirs(resource_dir, exist_ok=True)
            for event_object, event_details in event_objects.items():
                object_filepath = os.path.join(resource_dir, event_object)
                with open(object_filepath + '.yaml', 'w') as fp:
                    yaml.safe_dump(event_details, default_flow_style=False, stream=fp)

    # write_yaml writes yaml for given k8s object
    def write_yaml(self, resource_dir, object):
        obj_namespace = object['metadata'].get('namespace')
        obj_name = object['metadata']['name']
        resource_dir = os.path.join(self.output_dir, resource_dir, obj_namespace if obj_namespace else '.')
        os.makedirs(resource_dir, exist_ok=True)

        object_filepath = os.path.join(resource_dir, obj_name)
        with open(object_filepath + '.yaml', 'w') as fp:
            yaml.safe_dump(object, default_flow_style=False, stream=fp)

    # write_logs creates log for given pod object
    def write_logs(self, resource_dir, object):
        obj_namespace = object['metadata']['namespace']
        obj_name = object['metadata']['name']
        resource_dir = os.path.join(self.output_dir, resource_dir, obj_namespace)
        os.makedirs(resource_dir, exist_ok=True)

        containers = get_containers(object)
        for name, statuses in containers.items():
            if statuses[0]:
                self.write_log(resource_dir, obj_namespace, obj_name, name)

            if statuses[1]:
                self.write_log(resource_dir, obj_namespace, obj_name, name, True)


    # write_log writes logs of a pod object
    def write_log(self, resource_dir, obj_namespace, obj_name, container, is_previous=False):
        object_filepath = os.path.join(resource_dir, obj_name) + '.{}.{}.log'.format(container, 'prev' if is_previous else 'curr')
        with open(object_filepath, 'w') as fp:
            obj_path = '/api/v1/namespaces/{}/pods/{}/log?container={}&previous={}'.format(obj_namespace, obj_name,
                                                                                           container, is_previous)
            try:
                data = self.call(obj_path, 'GET')
                fp.write(data)
            except Exception as e:
                result = str(e).find("ContainerCreating")
                if result != -1:
                    log.info("API calling Failed for Object Path: "+obj_path)
                else:
                    raise

    # zipdir creates zip directory of collected info
    def zipdir(self):
        with zipfile.ZipFile(self.output_dir + '.zip', 'w', zipfile.ZIP_DEFLATED) as zipf:
            for root, dirs, files in os.walk(self.output_dir):
                for file in files:
                    zipf.write(os.path.join(root, file))
        if self.clean_output:
            shutil.rmtree(self.output_dir, ignore_errors=True)


# aggregate_events aggregates events based on involved objects
def aggregate_events(events, resource_map):
    event_dict = defaultdict(lambda: defaultdict(list))
    for event in events:
        involvedObject = event.get('involvedObject', {})
        apiVersion = involvedObject.get('apiVersion', '')
        kind = involvedObject.get('kind', '')
        namespace = involvedObject.get('namespace', '')
        name = involvedObject.get('name','')
        resource_nsnm = get_namespaced_name(namespace, name)

        if apiVersion.startswith(TRILIOVAULT_GROUP) or (kind in resource_map and resource_nsnm in resource_map[kind]):
            event.pop('metadata', None)
            event.pop('involvedObject', None)
            event_dict[namespace][f'{kind.lower()}\{name}'].append(event)
    return event_dict

# get_namespaced_name returns namespaced name representation of a resource
def get_namespaced_name(namespace, name):
    return f'{namespace}/{name}'

# filter_k8s_jobs returns list of jobs created by triliovault application
def filter_k8s_jobs(k8s_jobs):
    restore_operations = ['metadata-restore-validation', 'data-restore', 'metadata-restore']
    filter_k8s_job_list = []
    for k8s_job in k8s_jobs:
        ownerReferences = k8s_job['metadata'].get('ownerReferences', [])
        annotations = k8s_job['metadata'].get('annotations', {})
        if annotations.get('operation', '') in restore_operations:
            filter_k8s_job_list.append(k8s_job)
            continue

        for owner in ownerReferences:
            if owner.get('controller') and owner['apiVersion'].startswith('triliovault.trilio.io'):
                filter_k8s_job_list.append(k8s_job)
                break

    return filter_k8s_job_list


# filter_csv returns list of openshift csv created by triliovault
def filter_csv(csv_objects):
    filtered_csv_objects = []
    for csv_object in csv_objects:
        if csv_object['metadata']['name'].startswith('k8s-triliovault'):
            filtered_csv_objects.append(csv_object)

    return filtered_csv_objects


# filter_crd returns list of crds created by given set of groups
def filter_crd(crd_objects):
    crd_filter_group = [TRILIOVAULT_GROUP, SNAPSHOT_STORAGE_GROUP, CSI_STORAGE_GROUP]

    filtered_crd_objects = []
    for crd_object in crd_objects:
        if crd_object['spec']['group'] in crd_filter_group:
            filtered_crd_objects.append(crd_object)

    return filtered_crd_objects


# filter_pods returns list of pods created by triliovault application
def filter_pods(pod_objects, job_objects):

    pod_job_names = []
    for job in job_objects:
        pod_job_names.append(job['metadata']['name'])

    filter_pod_objects = []
    filter_pod_names = []
    for pod in pod_objects:
        pod_name = pod['metadata']['name']
        pod_ns = pod['metadata'].get('namespace', '')
        ownerReferences = pod['metadata'].get('ownerReferences', [])
        pod_ns_nm = get_namespaced_name(pod_ns, pod_name)

        controller_owner = None
        for owner in ownerReferences:
            if owner.get('controller') and owner.get('apiVersion') == 'batch/v1' and owner.get('kind') == 'Job':
                controller_owner = owner['name']

        if pod_name.startswith('k8s-triliovault') or controller_owner in pod_job_names:
            filter_pod_names.append(pod_ns_nm)
            filter_pod_objects.append(pod)

    return filter_pod_names, filter_pod_objects


# get_gv_list_by_group returns group_version matched for given group
def get_gv_list_by_group(api_gv_list, group):
    return [group_version for group_version in api_gv_list if group_version.startswith(group)]

# get_gv_by_group returns group_version matched for given group
def get_gv_by_group(api_gv_list, group_name, isPreferredVersion = True):
    for group in api_gv_list:
        if group.name == group_name:
            if isPreferredVersion:
                return group.preferred_version.group_version
            else:
                return [version.group_version for version in group.versions]
    return ''

# get_resource_gv_by_name returns resource object and gv for given resource name
def get_resource_gv_by_name(resource_map, name):
    gv_resource_map = dict()
    for gv, resources in resource_map.items():
        for resource in resources:
            if resource.name == name:
                gv_resource_map[gv] = resource
                continue

    return gv_resource_map

# get_resource_by_name returns resource object for given resource name
def get_resource_by_name(resources, name):
    matched_resources = [resource for resource in resources if resource.name == name]
    if matched_resources:
        return matched_resources[0]


# get_containers returns containers of a pod with their current and previous statuses
def get_containers(pod):
    containers = dict()
    container_statuses = pod.get('status', {}).get('containerStatuses', [])
    for container_status in container_statuses:
        containers[container_status['name']] = get_container_status_value(container_status)

    container_statuses = pod.get('status', {}).get('initContainerStatuses', [])
    for container_status in container_statuses:
        containers[container_status['name']] = get_container_status_value(container_status)

    return containers


# get_container_status_value returns wherether current and previous container present to capture logs
def get_container_status_value(container_status):
    current_container, previous_container = False, False
    last_state = container_status.get('lastState')
    current_state = container_status.get('state')
    if 'terminated' in last_state:
        previous_container = True
    if 'running' in current_state or 'terminated' in current_state:
        current_container = True

    return current_container, previous_container


# get_object_names returns list of names of objects
def get_object_names(objects):
    name_list = []
    for object in objects:
        name_list.append(object['metadata'].get('name'))

    return name_list

# get_api_group_version_resource_path returns api resource path for given group_version
def get_api_group_version_resource_path(api_group_version):
    if api_group_version == 'v1':
        return '/api/v1'
    else:
        return '/apis/' + api_group_version


if __name__ == '__main__':
    main()

