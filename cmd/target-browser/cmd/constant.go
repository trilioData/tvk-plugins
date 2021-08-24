package cmd

import (
	"fmt"
	"strings"

	"github.com/trilioData/tvk-plugins/internal"
)

const (
	KubeConfigFlag  = "kubeconfig"
	kubeConfigUsage = "Path to the kubeconfig file to use for CLI requests"

	InsecureSkipTLSFlag  = "insecure-skip-tls-verify"
	insecureSkipTLSUsage = "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure"

	UseHTTPS      = "use-https"
	useHTTPSUsage = "use https scheme for client connection"

	CertificateAuthorityFlag  = "certificate-authority"
	certificateAuthorityUsage = "Path to a cert file for the certificate authority"

	OutputFormatFlag      = "output"
	OutputFormatFlagShort = "o"

	TargetNameFlag  = "target-name"
	targetNameUsage = "Name of target CR to be used to query mounted target"

	TargetNamespaceFlag      = "target-namespace"
	targetNamespaceDefault   = "default"
	targetNamespaceUsage     = "Namespace of specified target CR to be used to query mounted target"
	BackupPlanCmdName        = "backupplan"
	backupPlanCmdPluralName  = BackupPlanCmdName + "s"
	backupPlanCmdAlias       = "backupPlan"
	backupPlanCmdAliasPlural = backupPlanCmdAlias + "s"
	BackupCmdName            = "backup"
	backupCmdPluralName      = BackupCmdName + "s"
	MetadataCmdName          = "metadata"
	ResourceMetadataCmdName  = "resource-metadata"

	pagesFlag    = "pages"
	pagesDefault = 1
	pagesUsage   = "Number of Pages to display within the paginated result set"

	PageSizeFlag = "page-size"

	PageSizeDefault = 10
	pageSizeUsage   = "Maximum number of results in a single page"

	OrderByFlag    = "order-by"
	orderByDefault = "name"
	orderByUsage   = "Parameter to use for ordering the paginated result set"

	TvkInstanceUIDFlag    = "tvk-instance-uid"
	tvkInstanceUIDDefault = ""

	tvkInstanceUIDUsage = "TVK instance id to filter backupPlan"

	BackupPlanUIDFlag    = "backup-plan-uid"
	backupPlanUIDDefault = ""
	backupPlanUIDUsage   = "backupPlanUID to get all backup related to UID"

	BackupStatusFlag    = "backup-status"
	backupStatusDefault = ""
	backupStatusUsage   = "Status of Backup to filter for [Success, InProgress, Failed]"

	BackupUIDFlag    = "backup-uid"
	backupUIDDefault = ""
	backupUIDUsage   = "backupUID to get all backup related to UID"

	creationDateFlag    = "creation-date"
	creationDateDefault = ""
	creationDateUsage   = "Backup creation date"

	expiryDateFlag    = "expiry-date"
	expiryDateDefault = ""
	expiryDateUsage   = "Backup expiry date"

	OperationScopeFlag  = "operation-scope"
	operationScopeUsage = "Filter backup/backupPlan for [SingleNamespace, MultiNamespace]. " +
		"Supported values can be in any case capital, small or mixed."

	groupFlag      = "group"
	groupFlagShort = "g"
	groupDefault   = ""
	groupUsage     = "API group name of resource whose resource-metadata needs to be retrieved"

	versionFlag      = "version"
	versionFlagShort = "v"
	versionDefault   = ""
	versionUsage     = "API version of resource whose resource-metadata needs to be retrieved"

	kindFlag      = "kind"
	kindFlagShort = "k"
	kindDefault   = ""
	kindUsage     = "API resource Kind of backed up resource whose resource-metadata needs to be retrieved"

	nameFlag    = "name"
	nameDefault = ""
	nameUsage   = "name of backed up resource whose resource-metadata needs to be retrieved"
)

var (
	tvkInstanceUID  string
	backupPlanUID   string
	group           string
	version         string
	kind            string
	name            string
	backupStatus    string
	backupUID       string
	creationDate    string
	expiryDate      string
	orderBy         string
	operationScope  string
	pages, pageSize int
)

var (
	OutputFormatFlagUsage = fmt.Sprintf("Output format to use. Supported formats: %s.",
		strings.Join(internal.AllowedOutputFormats.List(), "|"))
)
