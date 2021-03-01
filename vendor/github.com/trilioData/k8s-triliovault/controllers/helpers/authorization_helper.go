package helpers

import (
	"context"
	"errors"
	"strings"

	"github.com/trilioData/k8s-triliovault/internal/helpers"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/trilioData/k8s-triliovault/internal"
)

// TODO: Update based on securitycontextconstraints
var (
	openShiftPolicyRule = rbacv1.PolicyRule{
		APIGroups:     []string{"security.openshift.io"},
		ResourceNames: []string{"privileged"},
		Resources:     []string{"securitycontextconstraints"},
		Verbs:         []string{"use"},
	}
	restConfig = ctrl.GetConfigOrDie()
)

func getServiceAccountTemplate(name, namespace string) *corev1.ServiceAccount {
	serviceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       internal.ServiceAccountKind,
			APIVersion: corev1.GroupName,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return serviceAccount
}

func createClusterRoleTemplate(name string, policyRules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	clusterRole := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       internal.ClusterRoleKind,
			APIVersion: rbacv1.GroupName,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: policyRules,
	}

	return clusterRole
}

func getBackupPolicyRuleList() []rbacv1.PolicyRule {

	verbs := []string{internal.GET, internal.LIST, internal.WATCH}

	tvPolicyRule := rbacv1.PolicyRule{
		APIGroups: []string{internal.TrilioVaultGroup},
		Resources: []string{rbacv1.ResourceAll},
		Verbs:     []string{rbacv1.VerbAll},
	}

	podExecPolicyRule := rbacv1.PolicyRule{
		APIGroups: []string{internal.CoreGroup},
		Resources: internal.PodExecResource,
		Verbs:     []string{internal.CREATE},
	}

	CrdCreateRule := rbacv1.PolicyRule{
		APIGroups: []string{apiextensions.GroupName},
		Resources: []string{internal.CrdResource},
		Verbs:     []string{internal.CREATE},
	}

	// TODO: Update this once we segregate clusterRoles for each job
	allPolicyRule := rbacv1.PolicyRule{
		APIGroups: []string{rbacv1.APIGroupAll},
		Resources: []string{rbacv1.ResourceAll},
		Verbs:     verbs,
	}
	policyRules := []rbacv1.PolicyRule{tvPolicyRule, allPolicyRule, podExecPolicyRule, CrdCreateRule}
	if helpers.CheckIsOpenshift(restConfig) {
		policyRules = append(policyRules, openShiftPolicyRule)
	}
	return policyRules
}

func getRestorePolicyRoleList() []rbacv1.PolicyRule {

	verbs := []string{internal.GET, internal.LIST, internal.WATCH, internal.CREATE, internal.PATCH, internal.UPDATE}

	authPolicyRule := rbacv1.PolicyRule{
		APIGroups: []string{internal.AuthorizationGroup},
		Resources: []string{rbacv1.ResourceAll},
		Verbs:     []string{internal.ESCALATE, internal.BIND},
	}

	tvPolicyRule := rbacv1.PolicyRule{
		APIGroups: []string{internal.TrilioVaultGroup},
		Resources: []string{rbacv1.ResourceAll},
		Verbs:     []string{rbacv1.VerbAll},
	}

	// TODO: Update this for CRs of Custom CRDs
	allPolicyRule := rbacv1.PolicyRule{
		APIGroups: []string{rbacv1.APIGroupAll},
		Resources: []string{rbacv1.ResourceAll},
		Verbs:     verbs,
	}
	policyRules := []rbacv1.PolicyRule{tvPolicyRule, allPolicyRule, authPolicyRule}
	if helpers.CheckIsOpenshift(restConfig) {
		policyRules = append(policyRules, openShiftPolicyRule)
	}
	return policyRules
}

func getClusterRoleBindingTemplate(name string, role *rbacv1.ClusterRole, sa *corev1.ServiceAccount) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     internal.ClusterRoleKind,
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      internal.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		}},
	}

	return clusterRoleBinding
}

func GetAuthResourceName(resourceUID types.UID, kind string) string {
	authResourceName := internal.CategoryTriliovault + "-" + kind + "-" + string(resourceUID)

	return strings.ToLower(authResourceName)
}

func SetupRBACAuthorization(ctx context.Context, cli client.Client, resourceUID types.UID, namespace, kind string) error {

	if kind != internal.BackupKind && kind != internal.RestoreKind {
		return errors.New("kind Not supported for RBAC Authorization setup")
	}

	policyRuleList := getBackupPolicyRuleList()
	if kind == internal.RestoreKind {
		policyRuleList = getRestorePolicyRoleList()
	}

	authResourceName := GetAuthResourceName(resourceUID, kind)

	recommendedLabels := internal.GetRecommendedLabels(authResourceName, internal.ManagedBy)
	serviceAccount := getServiceAccountTemplate(authResourceName, namespace)
	serviceAccount.Labels = recommendedLabels
	err := cli.Create(ctx, serviceAccount)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRole := createClusterRoleTemplate(authResourceName, policyRuleList)
	clusterRole.Labels = recommendedLabels
	cErr := cli.Create(ctx, clusterRole)
	if cErr != nil && !apierrors.IsAlreadyExists(err) {
		return cErr
	}

	clusterRoleBinding := getClusterRoleBindingTemplate(authResourceName, clusterRole, serviceAccount)
	clusterRoleBinding.Labels = recommendedLabels
	clErr := cli.Create(ctx, clusterRoleBinding)
	if clErr != nil && !apierrors.IsAlreadyExists(err) {
		return clErr
	}
	return nil
}

func TearDownRBACAuthorization(ctx context.Context, cli client.Client, resourceUID types.UID, namespace, kind string) error {

	if kind != internal.BackupKind && kind != internal.RestoreKind {
		return errors.New("kind Not supported for RBAC Authorization setup")
	}

	authResourceName := GetAuthResourceName(resourceUID, kind)

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: authResourceName}}
	clErr := cli.Delete(ctx, clusterRoleBinding)
	if clErr != nil && !apierrors.IsNotFound(clErr) {
		return clErr
	}

	clusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: authResourceName}}
	cErr := cli.Delete(ctx, clusterRole)
	if cErr != nil && !apierrors.IsNotFound(clErr) {
		return cErr
	}

	serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: authResourceName, Namespace: namespace}}
	err := cli.Delete(ctx, serviceAccount)
	if err != nil && !apierrors.IsNotFound(clErr) {
		return err
	}

	return nil
}
