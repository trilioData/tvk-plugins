package helpers

import (
	"context"

	"github.com/trilioData/k8s-triliovault/internal"
	v1 "k8s.io/api/apps/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TvkHealthSummary []DeploymentSummary

type List v1.DeploymentList
type DeploymentSummary struct {
	Deployment string `json:"deployment"`
	Namespace  string `json:"namespace"`
	Total      int32  `json:"total"`
	Running    int32  `json:"running"`
}

// CalculateSummary, Function for Creating TvkHealthSummary Object
func (list *List) CalculateSummary() (TvkHealthSummary, error) {
	var tvkHealthSummary TvkHealthSummary
	for idx := range list.Items {
		deployment := list.Items[idx]
		// For Namespace scoped installation Trilio resources only from installation namespace
		if internal.GetAppScope() == internal.NamespacedScope &&
			!internal.ContainsString([]string{internal.GetInstallNamespace()}, deployment.Namespace) {
			continue
		}
		tvkHealthSummary = append(tvkHealthSummary, DeploymentSummary{
			Deployment: deployment.Name,
			Namespace:  deployment.Namespace,
			Total:      deployment.Status.Replicas,
			Running:    deployment.Status.ReadyReplicas,
		})
	}
	return tvkHealthSummary, nil
}
func GetTvkHealthSummary(ctx context.Context, cli client.Client) (TvkHealthSummary, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetDeploymentList")
	// Getting Deployments List, Filter by labels
	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"app.kubernetes.io/name": internal.PartOf},
	}
	deploymentsList, err := GetDeploymentList(ctx, cli, labelSelector)
	if err != nil {
		log.Error(err, "Error while getting the deployment list from api cache")
		return nil, err
	}
	list := List(*deploymentsList)
	result, err := list.CalculateSummary()
	if err != nil {
		log.Error(err, "Error while creating the summary result")
		return nil, err
	}
	return result, nil
}

// function to get Deployments List
func GetDeploymentList(ctx context.Context, apiClient client.Client, labelSelector *metav1.LabelSelector) (*v1.DeploymentList, error) {
	log := ctrl.Log.WithName("function").WithName("common:GetDeploymentList")

	opts := &client.ListOptions{}
	// Adding labelSelector to filter it over labels
	if labelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(labelSelector)
		if err != nil {
			log.Error(err, "failed to convert LabelSelector api type into labels.Selector")
			return nil, err
		}
		opts.LabelSelector = selector
	}
	deploymentList := &v1.DeploymentList{}
	if err := apiClient.List(ctx, deploymentList, opts); err != nil {
		log.Error(err, "failed to get deploymentList from apiServer cache")
		return nil, err
	}
	return deploymentList, nil
}
