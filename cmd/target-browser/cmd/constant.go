package cmd

const (
	kubeConfigFlag  = "kubeconfig"
	kubeConfigUsage = "Path to the kubeconfig file to use for CLI requests"

	insecureSkipTLSFlag  = "insecure-skip-tls-verify"
	insecureSkipTLSUsage = "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure"

	clientCertificateFlag  = "client-certificate"
	clientCertificateUsage = "Path to a client certificate file for TLS"

	clientKeyFlag  = "client-key"
	clientKeyUsage = "Path to a client key file for TLS"

	certificateAuthorityFlag  = "certificate-authority"
	certificateAuthorityUsage = "Path to a cert file for the certificate authority"

	targetNameFlag  = "target-name"
	targetNameUsage = "Name of target CR to be used to query mounted target"

	targetNamespaceFlag    = "target-namespace"
	targetNamespaceDefault = "default"
	targetNamespaceUsage   = "Namespace of specified target CR to be used to query mounted target"

	backupPlanCmdName        = "backupplan"
	backupPlanCmdPluralName  = backupPlanCmdName + "s"
	backupPlanCmdAlias       = "backupPlan"
	backupPlanCmdAliasPlural = backupPlanCmdAlias + "s"
	backupCmdName            = "backup"
	backupCmdPluralName      = backupCmdName + "s"
	metadataCmdName          = "metadata"

	pagesFlag    = "Pages"
	pagesDefault = 1
	pagesUsage   = "Number of Pages to display within the paginated result set"


	shortUsage           = "Retrieve a list of BackupPlans in that target store."
	longUsage            = `Retrieve a list of BackupPlans in that target store using options.
		Order backupPlan in ascending or descending order,
		Filter backupPlan using flag tvk-instance-uid.`

	backupShortUsage = "Retrieve a list of Backup in that target store."
	backupLongUsage  = `Retrieve a list of Backup in that target store for specific backupPlan.
			Filter backup using flag backup-status,
			Order backup  in ascending or descending order.`

	PageSizeFlag = "page-size"
	MetadataBinaryName = "metadata"
	pageSizeDefault = 10
	pageSizeUsage   = "Maximum number of results in a single page"
	pageSizeShort   = "s"

	orderByFlag    = "order-by"
	orderByDefault = "name"
	orderByUsage   = "Parameter to use for ordering the paginated result set"

	metadataShortUsage = "API to perform Read operations on Backup high level metadata"
	metadataLongUsage  = "API to perform Read operations on Backup high level metadata"

	pageFlag    = "page"
	pageDefault = 1
	pageShort   = "p"
	pageUsage   = "A page number within the paginated result set."

	OrderingFlag    = "ordering"
	orderingDefault = "name"
	orderingShort   = "o"
	orderingUsage   = "ordering"

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
	pageSize       int
	page           int
	ordering       string
	tvkInstanceUID string
	backupPlanUID  string
	backupStatus   string
	backupUID      string
	APIKey         = ""
)
