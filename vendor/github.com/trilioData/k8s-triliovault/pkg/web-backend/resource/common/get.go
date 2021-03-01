package common

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emicklei/go-restful"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels2 "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
)

const (
	AuthClient = "AuthClient"

	PutRequestMethod = "PUT"
)

var UnAuthenticatedURLs = []string{"/login", "/token/refresh", "/readyz"}

// Paginator struct for pagination
type Paginator struct {
	Page      int `json:"page"`
	PageSize  int `json:"pageSize"`
	ResultLen int `json:"resultLen"`
	To        int `json:"to"`
	From      int `json:"from"`
	Next      int `json:"next"`
}

// Sorter struct for sorting
type Sorter struct {
	Order       string `json:"orderingType"`
	ArrangeType string `json:"arrangeType"`
}

// ListMetadata for Pagination handling at backend
type ListMetadata struct {
	Total int `json:"total"`
	Next  int `json:"next"`
}

type OrderingType string

const (
	name              OrderingType = "name"
	creationTimestamp OrderingType = "creationTimestamp"
	capacityAvailable OrderingType = "capacityAvailable"
	capacityOccupied  OrderingType = "capacityOccupied"
	size              OrderingType = "size"
)

const (
	ASC  = "ascending"
	DESC = "descending"

	// Required values for recommended labels
	ManagedBy    = "k8s-triliovault-ui"
	PartOf       = "k8s-triliovault"
	DefaultLabel = "k8s-triliovault"

	Group     = "group"
	Version   = "version"
	Kind      = "kind"
	Verb      = "verb"
	Name      = "name"
	Namespace = "namespace"

	Search = "search"
)

// Function for setting pagination & Ordering struct
func InitRequestParams(request *restful.Request) (*Paginator, *Sorter) {
	paginator := &Paginator{}

	paginator.Page, paginator.PageSize, _ = PaginationValidate(request)

	sorter := &Sorter{}
	sorter.Set(request)

	return paginator, sorter
}

// Function for Paginator to calculate To, From & Next
func (paginator *Paginator) Set(resultLen int) error {
	paginator.ResultLen = resultLen
	err := paginator.getPaginationParams()
	return err
}

// Function for Sorter to setup
func (sorter *Sorter) Set(request *restful.Request) {
	orderingType := request.QueryParameter("ordering")
	tmpOrderingType := orderingType
	if orderingType != "" {
		sorter.ArrangeType = ASC
		if string(orderingType[0]) == "-" {
			sorter.ArrangeType = DESC
			tmpOrderingType = orderingType[1:]
		}
		sorter.Order = tmpOrderingType
	}
}

// Function for Validating Ordering Type
func (orderingType OrderingType) Validate() bool {
	orderingTypeMap := map[OrderingType]bool{name: true, creationTimestamp: true, capacityOccupied: true, capacityAvailable: true, size: true}
	return orderingTypeMap[orderingType]
}

// Function for getting pagination parameters after calculation like to, from
func (paginator *Paginator) getPaginationParams() error {
	pageSize, page, resultLen := paginator.PageSize, paginator.Page, paginator.ResultLen
	if pageSize == 0 {
		pageSize = 10
	}
	if (pageSize*page)-pageSize > resultLen {
		return errors.New("not a valid page")
	}

	var to, from, next int
	if page > 0 {
		from = (page * pageSize) - pageSize
		if (page*pageSize)-1 <= resultLen {
			to = (page * pageSize)

			if to > resultLen {
				to = resultLen
			}
		} else {
			to = resultLen
		}

		if page*pageSize >= resultLen {
			next = -1
		} else {
			next = page + 1
		}
	} else {
		from = 0
		if pageSize-1 <= resultLen {
			to = pageSize - 1
		} else {
			to = resultLen
		}
	}

	paginator.To, paginator.From, paginator.Next = to, from, next
	return nil
}

// Function for validating Pagination request parameters
func PaginationValidate(request *restful.Request) (page, pageSize int, err error) {
	log := ctrl.Log.WithName("function").WithName("common:PaginationValidate")
	// Getting parameters for pagination from Request
	pageSizeStr := request.QueryParameter("pageSize")
	if pageSizeStr != "" {
		pageSize, err = strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 0 {
			log.Error(err, "not a valid pageSize parameter")
			return 0, 0, errors.New("page size parameter is not valid")
		}
	}

	pageStr := request.QueryParameter("page")
	if pageStr != "" {
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			log.Error(err, "not a valid page parameter")
			return 0, 0, errors.New("page parameter is not valid")
		}
	} else {
		page = 1
	}

	return page, pageSize, nil
}

// Function Responsible for Sorting runtime objects
func SortingObjects(objs []runtime.Object, orderType string) ([]runtime.Object, error) {
	sorter := NewRuntimeSorter(objs, orderType)
	if err := sorter.Sort(); err != nil {
		return nil, err
	}
	return sorter.Objects, nil
}

// Function responsible for Generic Sorting
func SortingByOrderingType(unsortedObjs []runtime.Object, orderingType string) ([]runtime.Object, error) {
	var objs []runtime.Object
	if string(orderingType[0]) == "-" {
		orderingType = orderingType[1:]
	}
	var err error
	switch orderingType {
	case "name":
		objs, err = SortingObjects(unsortedObjs, ".metadata.name")
	case "creationTimestamp":
		objs, err = SortingObjects(unsortedObjs, ".metadata.creationTimestamp")
	case "capacityAvailable":
		objs, err = SortingObjects(unsortedObjs, ".generatedField.capacityAvailable")
	case "capacityOccupied":
		objs, err = SortingObjects(unsortedObjs, ".generatedField.capacityOccupied")
	}
	if err != nil {
		return nil, err
	}

	return objs, nil
}

func ValidateParameters(request *restful.Request) error {
	log := ctrl.Log.WithName("function").WithName("common:ValidateParameters")
	_, _, err := PaginationValidate(request)
	if err != nil {
		log.Info("not a valid pagination parameters", "error", err)
		return err
	}

	return nil
}

func ContainsApplicationType(slice []v1.ApplicationType, s v1.ApplicationType) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func ContainsStatus(slice []v1.Status, s v1.Status) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// Function to Get Object Key for Getting objects from apiServer Cache
func GetObjectKey(name, namespace string) types.NamespacedName {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	return key
}

// Function to Get Object Meta for Deleting objects from apiServer Cache
func GetObjectMeta(name, namespace string) metav1.ObjectMeta {
	objectMeta := metav1.ObjectMeta{Name: name, Namespace: namespace}
	return objectMeta
}

// Equal tells whether a and b contain the same elements.
// A nil argument is equivalent to an empty slice.
func IsSliceOfStringEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Sorting a & b slice
	sort.Strings(a)
	sort.Strings(b)

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// UniqueSliceOfString returned a unique list of Strings by removing duplicate values
func UniqueSliceOfString(list []string) []string {
	isMap := map[string]bool{}
	var resultList []string
	for idx := range list {
		item := list[idx]
		if !isMap[item] {
			isMap[item] = true
			resultList = append(resultList, item)
		}
	}
	return resultList
}

func IsBackupPlanApplicationFilter(namespaceBackupPlanMap map[string]map[string]v1.BackupPlan, backupPlan *v1.BackupPlan,
	appFilter *ApplicationSelectorSearchFilter) bool {
	aggregatedApps := getAggregatedApplicationsSearchMeta(appFilter)
	backupPlanComponents := backupPlan.Spec.BackupPlanComponents

	// Checking if Labels matches with the BackupPlan
	for idx := range appFilter.Labels {
		label := appFilter.Labels[idx]
		labelMap := map[string]string{label.Key: label.Value}
		if !IsSelectorMatches(backupPlanComponents.Custom, metav1.LabelSelector{MatchLabels: labelMap}) {
			return false
		}
	}
	// Checking if Objects's labels matches with the BackupPlan
	for idx := range appFilter.Objects {
		labelMap := appFilter.Objects[idx].Object.Labels

		if !IsSelectorMatches(backupPlanComponents.Custom, metav1.LabelSelector{MatchLabels: labelMap}) {
			return false
		}
	}

	// Iterating over BackupPlan components to aggregate and Filter backupPlan over requested labels & MatchExpression
	//	1. Iterating over LabelSelector to check for Every requested label Matches or not
	// 	2. If this component have helmCharts then it will Append to the Aggregated list of Helms
	//	3. If This component have Operator's Helm Chart then it will append to Aggregated list of Operators
	for idx := range appFilter.BackupPlans {
		requestBackupPlanName := appFilter.BackupPlans[idx]

		requestBackupPlan := namespaceBackupPlanMap[requestBackupPlanName.Namespace][requestBackupPlanName.Name]
		if requestBackupPlan.Name == "" {
			return false
		}

		// Namespace of requested BackupPlan and of matched BackupPlan should match
		if backupPlan.GetNamespace() != requestBackupPlan.GetNamespace() {
			return false
		}

		// Checking if this Request's backupPlan is for NamespaceBackup
		if IsNamespaceBackup(&requestBackupPlan) && !IsNamespaceBackup(backupPlan) {
			return false
		}

		if !IsNamespaceBackup(&requestBackupPlan) {
			component := requestBackupPlan.Spec.BackupPlanComponents
			if len(component.Custom) == 0 && len(backupPlanComponents.Custom) > 0 {
				return false
			}
			// Iterating over LabelSelector to check if it matches with the requested BackupPlan Component
			for idx1 := range component.Custom {
				labelSelector := component.Custom[idx1]
				if labelSelector.Size() == 0 {
					continue
				}
				if !IsSelectorMatches(backupPlanComponents.Custom, labelSelector) {
					return false
				}
			}

			// Appending if it have HelmReleases
			if len(component.HelmReleases) > 0 {
				aggregatedApps.HelmReleases = append(aggregatedApps.HelmReleases, component.HelmReleases...)
			}

			// Checking if BackupPlanComponents doesn't have any operator
			if len(component.Operators) > 0 && len(backupPlanComponents.Operators) == 0 {
				return false
			}
			// Iterating over Operators to get Operators' HelmRelease
			if len(component.Operators) > 0 {
				aggregatedApps.Operators = append(aggregatedApps.Operators, GetOperatorHelmCharts(component)...)
			}
		}
	}

	// Removing Duplicate values from helmReleases & Operators if Any
	aggregatedApps.HelmReleases = UniqueSliceOfString(aggregatedApps.HelmReleases)
	aggregatedApps.Operators = UniqueSliceOfString(aggregatedApps.Operators)

	// Checking if all the Helm Releases Exists in the BackupPlan
	if !IsSliceOfStringEqual(aggregatedApps.HelmReleases, backupPlanComponents.HelmReleases) {
		return false
	}

	// Checking if all the Operators Helm Charts Exists in the backupPlan
	if !IsSliceOfStringEqual(aggregatedApps.Operators, GetOperatorHelmCharts(backupPlanComponents)) {
		return false
	}

	// Checking if length of components are same in both given components and backupplan components
	if !isComponentsLengthEquals(backupPlanComponents, appFilter) {
		return false
	}

	return true
}

func isComponentsLengthEquals(components v1.BackupPlanComponents, appFilter *ApplicationSelectorSearchFilter) bool {
	// Checking if only helms is given
	if (len(appFilter.Objects) == 0 && len(appFilter.Labels) == 0 && len(appFilter.BackupPlans) == 0) &&
		len(components.Custom) != 0 {
		return false
	}
	return true
}

func getAggregatedApplicationsSearchMeta(appFilter *ApplicationSelectorSearchFilter) AggregatedApplicationSearchFilter {
	var helmReleases []string
	var operators []string

	// Iterating over Applications to aggregate the App detail
	for idx := range appFilter.Applications {
		app := appFilter.Applications[idx]
		switch app.Type {
		case v1.HelmType:
			helmReleases = append(helmReleases, app.Name)
		case v1.OperatorType:
			operators = append(operators, app.Name)
		}
	}

	return AggregatedApplicationSearchFilter{
		HelmReleases: helmReleases,
		Operators:    operators,
	}
}

func GetOperatorHelmCharts(backupPlanComponent v1.BackupPlanComponents) []string {
	var operators []string
	for idx1 := range backupPlanComponent.Operators {
		operator := backupPlanComponent.Operators[idx1]
		if operator.HelmRelease != "" {
			operators = append(operators, operator.HelmRelease)
		}
	}
	return operators
}

// Function for checking given selector matches with the any of LabelSelector from the Given List of LabelSelector
func IsSelectorMatches(custom []metav1.LabelSelector, requestSelector metav1.LabelSelector) bool {
	ls := labels2.Set(requestSelector.MatchLabels)
	for idx := range custom {
		selector := custom[idx]
		labelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: selector.MatchLabels})
		if labelSelector.Empty() && len(ls) > 0 {
			return false
		}

		if labelSelector.Matches(ls) && checkRequirementMatches(selector, requestSelector.MatchExpressions) {
			return true
		}
	}
	return false
}

// Function for checking given lists of LabelSelector are equals
func IsSelectorListMatches(a, b []metav1.LabelSelector) bool {
	for idx := range a {
		if !IsSelectorMatches(b, a[idx]) {
			return false
		}
	}
	return true
}

// Function for checking If the requested list of Requirements are matches with the List of LabelSelector
func checkRequirementMatches(selector metav1.LabelSelector, requirements []metav1.LabelSelectorRequirement) bool {
	var isRequirementExist bool
	// Checking Edge case when requirement MatchExpression is Nil and The Selector has expression
	if len(requirements) == 0 && len(selector.MatchExpressions) > 0 {
		return false
	}
	for idx := range requirements {
		isRequirementExist = false
		for idx2 := range selector.MatchExpressions {
			if isSelectorRequirementEquals(selector.MatchExpressions[idx2], requirements[idx]) {
				isRequirementExist = true
			}
		}
		if !isRequirementExist {
			return false
		}
	}
	return true
}

// Function for checking if the given LabelSelectorRequirement is matches with the list of LabelSelector
func isSelectorRequirementEquals(a, b metav1.LabelSelectorRequirement) bool {
	// Checking if key is Identical
	if a.Key != b.Key {
		return false
	}

	// Checking if value is Identical
	if !IsSliceOfStringEqual(a.Values, b.Values) {
		return false
	}

	// Checking if Operator is Identical
	if a.Operator != b.Operator {
		return false
	}
	return true
}

// Function For checking if this BackupPlan is for NamespaceBackup
func IsNamespaceBackup(backupPlan *v1.BackupPlan) bool {
	component := backupPlan.Spec.BackupPlanComponents
	if len(component.Custom) == 0 && len(component.HelmReleases) == 0 && len(component.Operators) == 0 {
		return true
	}
	return false
}

// Function For checking the BackupPlan scope
func GetBackupPlanScope(backupPlan *v1.BackupPlan) v1.ComponentScope {
	component := backupPlan.Spec.BackupPlanComponents
	if len(component.Custom) == 0 && len(component.HelmReleases) == 0 && len(component.Operators) == 0 {
		return v1.Namespace
	}
	return v1.App
}

// Function responsible for checking If the given timestamp is in given range
func IsTimestampInRange(timeStamp metav1.Time, filter TimeRangeFilter) bool {
	var rangeTime time.Time
	value := filter.TimeRangeValue
	switch filter.TimeRangeField {
	case Hour:
		rangeTime = time.Now().Add(-time.Duration(value) * time.Hour)
	case Day:
		rangeTime = time.Now().AddDate(0, 0, -value)
	case Week:
		rangeTime = time.Now().AddDate(0, 0, -7*value)
	case Month:
		rangeTime = time.Now().AddDate(0, -value, 0)
	}
	return timeStamp.After(rangeTime)
}

// IsBackupPlanProtected Checking if backupPlan is Protected or Not
func IsBackupPlanProtected(backupList []v1.Backup) bool {
	for idx := range backupList {
		if backupList[idx].Status.Status == v1.Available {
			return true
		}
	}
	return false
}

// GetErrorMap responsible for Making Error Map for UI
func GetErrorMap(err error) map[string]string {
	errSlice := strings.Split(err.Error(), ":")
	// Checking for generic error
	if len(errSlice) == 1 {
		return map[string]string{GenericError: errSlice[0]}
	}
	// Checking for Webhook error
	webhookErrorSlice := strings.Split(errSlice[len(errSlice)-1], "] ")
	if len(webhookErrorSlice) > 1 {
		key := strings.Replace(webhookErrorSlice[0], "[", "", 1)
		return map[string]string{strings.TrimSpace(key): strings.TrimSpace(webhookErrorSlice[1])}
	}
	// TO-DO for which Multiple Error comes with Generic errors(Schema Validation Error)
	if len(errSlice) >= 3 {
		return map[string]string{GenericError: strings.TrimSpace(strings.Join(errSlice[2:], ":"))}
	}
	return map[string]string{GenericError: strings.TrimSpace(errSlice[len(errSlice)-1])}
}

// GetRecommendedLabels, function for Getting Recommended Labels
func GetRecommendedLabels() map[string]string {
	recommendedLabels := map[string]string{
		"app.kubernetes.io/name":       DefaultLabel,
		"app.kubernetes.io/managed-by": ManagedBy,
		"app.kubernetes.io/part-of":    PartOf,
	}

	return recommendedLabels
}

// AddRecommendedLabels responsible for adding Recommended labels to Any Resource
func AddRecommendedLabels(labels map[string]string) map[string]string {
	recommendedLabels := GetRecommendedLabels()
	return MergeTwoMaps(labels, recommendedLabels)
}

// MergeTwoMaps, Function for merging two maps
func MergeTwoMaps(a, b map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range b {
		result[k] = v
	}
	for k, v := range a {
		result[k] = v
	}
	return result
}
