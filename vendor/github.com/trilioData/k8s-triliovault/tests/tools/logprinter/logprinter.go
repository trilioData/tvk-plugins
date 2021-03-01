package logprinter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/ghodss/yaml"
	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/internal/helpers"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ctx       = context.Background()
	scheme    *runtime.Scheme
	cfg       *rest.Config
	clientset kubernetes.Interface
	cl        client.Client
	InstallNs = os.Getenv("INSTALL_NAMESPACE")
	RestoreNs = os.Getenv("RESTORE_NAMESPACE")
)

func init() {
	cfg = ctrl.GetConfigOrDie()
	scheme = runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	cl, _ = client.New(cfg, client.Options{Scheme: scheme})
	clientset, _ = kubernetes.NewForConfig(cfg)
}

func PrintDebugLogs() {
	fmt.Println("###########################################################################################")

	crdsLists := []string{
		"Backup",
		"Restore",
		"Target",
		"BackupPlan",
	}

	for _, crds := range crdsLists {
		PrintCR(crds, InstallNs)
	}
}

func PrintCR(crds, ns string) {
	conditions := []string{"Error", "Failed", "Unavailable", "InProgress"}
	crdGvk := schema.GroupVersionKind{
		Kind:    crds,
		Version: internal.V1Version,
		Group:   internal.TrilioVaultGroup,
	}

	controlPlaneLabelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": "k8s-triliovault-control-plane"}}
	webhookServerLabelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": "k8s-triliovault-admission-webhook"}}

	cpPods, _ := clientset.CoreV1().Pods(ns).
		List(ctx, metav1.ListOptions{LabelSelector: labels.Set(controlPlaneLabelSelector.MatchLabels).String()})
	wsPods, _ := clientset.CoreV1().Pods(ns).
		List(ctx, metav1.ListOptions{LabelSelector: labels.Set(webhookServerLabelSelector.MatchLabels).String()})

	uList := &unstructured.UnstructuredList{}
	uList.SetGroupVersionKind(crdGvk)

	err := cl.List(context.Background(), uList, &client.ListOptions{Namespace: ns})
	if err != nil {
		panic(err)
	}

	for i, u := range uList.Items {
		status, _, err := unstructured.NestedString(u.Object, "status", "status")
		if err != nil {
			panic(err)
		}
		for _, con := range conditions {
			if status == con {
				fmt.Printf("Printing Debug Logs on %s with status %s\n\n", u.GetName(), status)
				if u.GetKind() == internal.BackupKind || u.GetKind() == internal.RestoreKind || u.GetKind() == internal.TargetKind {
					if len(cpPods.Items) > 0 {
						PrintLogs(cpPods.Items[0].Name, cpPods.Items[0].Namespace, 200)
					}
					if len(wsPods.Items) > 0 {
						PrintLogs(wsPods.Items[0].Name, wsPods.Items[0].Namespace, 200)
					}
				}
				PrettyPrintObj(&uList.Items[i])
				PrintChildObjects(&uList.Items[i], ns, cl)
			}
		}
	}

	fmt.Println("###########################################################################################")
}

func PrintChildObjects(owner *unstructured.Unstructured, ns string, cl client.Client) {
	fmt.Printf("Printing child jobs of %s named %s", owner.GetKind(), owner.GetName())

	var backup v1.Backup
	var restore v1.Restore
	var target v1.Target
	var backupPlan v1.BackupPlan
	var childJobs helpers.UnstructuredResourceList

	resList := getResourceList(internal.JobKind, ns)

	switch owner.GetKind() {
	case internal.BackupKind:
		err := cl.Get(context.Background(), types.NamespacedName{
			Namespace: ns,
			Name:      owner.GetName(),
		}, &backup)
		if err == nil {
			childJobs = resList.GetChildrenForOwner(&backup)
		} else {
			fmt.Println(err)
		}
	case internal.RestoreKind:
		err := cl.Get(context.Background(), types.NamespacedName{
			Namespace: ns,
			Name:      owner.GetName(),
		}, &restore)
		if err == nil {
			childJobs = getRestoreReference(&restore)
		} else {
			fmt.Println(err)
		}
	case internal.TargetKind:
		podList := getResourceList(internal.JobKind, ns)
		err := cl.Get(context.Background(), types.NamespacedName{
			Namespace: ns,
			Name:      owner.GetName(),
		}, &target)
		if err == nil {
			childJobs = podList.GetChildrenForOwner(&target)
		} else {
			fmt.Println(err)
		}
	case internal.BackupplanKind:
		err := cl.Get(context.Background(), types.NamespacedName{
			Namespace: ns,
			Name:      owner.GetName(),
		}, &backupPlan)
		if err == nil {
			childJobs = resList.GetChildrenForOwner(&backupPlan)
		} else {
			fmt.Println(err)
		}
	}

	for i, job := range childJobs.Items {
		PrettyPrintObj(&childJobs.Items[i])
		podList := getResourceList(internal.PodKind, job.GetNamespace())
		childPods := podList.GetChildrenForOwner(&childJobs.Items[i])

		for _, pod := range childPods.Items {
			PrintLogs(pod.GetName(), pod.GetNamespace(), 200)
		}
	}
}

func PrintLogs(name, namespace string, podLines int64) {

	fmt.Printf("Printing the %s pod logs\n\n", name)
	fmt.Println("==========================================================================================")
	podLogOpts := corev1.PodLogOptions{TailLines: &podLines}

	req := clientset.CoreV1().Pods(namespace).GetLogs(name, &podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		fmt.Println("error in opening stream")
	}
	defer func() {
		if podLogs != nil {
			_ = podLogs.Close()
		}
	}()

	if podLogs != nil {
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, podLogs)
		if err != nil {
			fmt.Printf("error in copy information from podLogs to buf")
		}

		fmt.Println(buf.String())
		fmt.Println("==========================================================================================")
	}
}

func PrettyPrintObj(u *unstructured.Unstructured) {
	fmt.Printf("Pretty Print the %s named %s", u.GetKind(), u.GetName())
	fmt.Println("*******************************************************************************************")
	// Removing the managed field before printing.
	unstructured.RemoveNestedField(u.Object, "metadata", "managedFields")
	y, err := yaml.Marshal(u.Object)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(y))
	fmt.Println("*******************************************************************************************")
}

func getResourceList(objType, ns string) helpers.UnstructuredResourceList {
	uPodList := &unstructured.UnstructuredList{}
	if objType == internal.PodKind {
		uPodList.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    internal.PodKind,
		})
	} else if objType == internal.JobKind {
		uPodList.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "batch",
			Version: "v1",
			Kind:    internal.JobKind,
		})
	}

	err := cl.List(context.Background(), uPodList, &client.ListOptions{Namespace: ns})
	if err != nil {
		fmt.Println("Error listing pods")
	}

	podList := helpers.UnstructuredResourceList(*uPodList)
	return podList
}

func getRestoreReference(owner runtime.Object) helpers.UnstructuredResourceList {
	children := helpers.UnstructuredResourceList{}
	restoreList := getResourceList(internal.JobKind, RestoreNs)
	metaOwner, err := meta.Accessor(owner)
	if err != nil {
		fmt.Printf("Error while converting the owner to meta accessor format %s\n", err)
	}
	for _, restore := range restoreList.Items {
		ann := restore.GetAnnotations()
		if ann["controller-owner-name"] == metaOwner.GetName() {
			children.Items = append(children.Items, restore)
		}
	}
	return children
}

func GetPodLogsFromJob(jobs *batchv1.Job) {
	podList := getResourceList(internal.PodKind, jobs.GetNamespace())
	childPods := podList.GetChildrenForOwner(jobs)

	for _, pod := range childPods.Items {
		PrintLogs(pod.GetName(), pod.GetNamespace(), 400)
	}
}

func PrintAvailableCR(crd string) {
	crdGvk := schema.GroupVersionKind{
		Kind:    crd,
		Version: internal.V1Version,
		Group:   internal.TrilioVaultGroup,
	}

	uList := &unstructured.UnstructuredList{}
	uList.SetGroupVersionKind(crdGvk)

	err := cl.List(context.Background(), uList, &client.ListOptions{Namespace: InstallNs})
	if err != nil {
		panic(err)
	}

	for i := range uList.Items {
		PrettyPrintObj(&uList.Items[i])
	}
}
