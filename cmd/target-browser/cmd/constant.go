package cmd

const (
	KubeConfigFlag  = "kubeconfig"
	kubeConfigUsage = "Path to the kubeconfig file to use for CLI requests"

	insecureSkipTLSFlag  = "insecure-skip-tls-verify"
	insecureSkipTLSUsage = "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure"

	clientCertificateFlag  = "client-certificate"
	clientCertificateUsage = "Path to a client certificate file for TLS"

	clientKeyFlag  = "client-key"
	clientKeyUsage = "Path to a client key file for TLS"

	certificateAuthorityFlag  = "certificate-authority"
	certificateAuthorityUsage = "Path to a cert file for the certificate authority"

	TargetNameFlag  = "target-name"
	targetNameUsage = "Name of target CR to be used to query mounted target"

	TargetNamespaceFlag      = "target-namespace"
	targetNamespaceDefault   = "default"
	targetNamespaceUsage     = "Namespace of specified target CR to be used to query mounted target"
	GetFlag                  = "get"
	BackupPlanCmdName        = "backupplan"
	backupPlanCmdPluralName  = BackupPlanCmdName + "s"
	backupPlanCmdAlias       = "backupPlan"
	backupPlanCmdAliasPlural = backupPlanCmdAlias + "s"
	BackupCmdName            = "backup"
	backupCmdPluralName      = BackupCmdName + "s"
	metadataCmdName          = "metadata"

	PagesFlag    = "Pages"
	pagesDefault = 1
	pagesUsage   = "Number of Pages to display within the paginated result set"

	backupPlanShortUsage = "API to perform Read operations on BackupPlans"
	backupPlanLongUsage  = `API to perform Read operations on BackupPlans. Get a list of BackupPlans from target with using options.
		Order backupPlan in ascending or descending order,
		Filter backupPlan using flag tvk-instance-uid.`

	backupShortUsage = "API to perform Read operations on Backup"
	backupLongUsage  = `API to perform Read operations on Backup. Get list of Backup stored on target for specific backupPlan.
			Filter backup using flag backup-status,
			Order backup  in ascending or descending order.`

	PageSizeFlag       = "page-size"
	MetadataBinaryName = "metadata"
	pageSizeDefault    = 10
	pageSizeUsage      = "Maximum number of results in a single page"

	OrderByFlag    = "order-by"
	orderByDefault = "name"
	orderByUsage   = "Parameter to use for ordering the paginated result set"

	metadataShortUsage = "API to perform Read operations on Backup high level metadata"
	metadataLongUsage  = `API to perform Read operations on Backup high level metadata.
		Get metadata of specific backup using flag backup-plan-uid and backup-uid`

	TvkInstanceUIDFlag    = "tvk-instance-uid"
	tvkInstanceUIDDefault = ""
	tvkInstanceUIDShort   = "t"
	tvkInstanceUIDUsage   = "TVK instance id to filter for."

	BackupPlanUIDFlag    = "backup-plan-uid"
	backupPlanUIDDefault = ""
	backupPlanUIDShort   = ""
	backupPlanUIDUsage   = "backupPlanUID to get all backup related to UID"

	BackupStatusFlag    = "backup-status"
	backupStatusDefault = ""
	backupStatusShort   = ""
	backupStatusUsage   = "Status of Backup to filter for [Success, InProgress, Failed]"

	BackupUIDFlag    = "backup-uid"
	backupUIDDefault = ""
	backupUIDShort   = ""
	backupUIDUsage   = "backupUID to get all backup related to UID"
)

var (
	tvkInstanceUID string
	backupPlanUID  string
	backupStatus   string
	backupUID      string
)
