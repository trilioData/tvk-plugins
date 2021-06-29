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
	BackupPlanCmdName        = "backupplan"
	backupPlanCmdPluralName  = BackupPlanCmdName + "s"
	backupPlanCmdAlias       = "backupPlan"
	backupPlanCmdAliasPlural = backupPlanCmdAlias + "s"
	BackupCmdName            = "backup"
	backupCmdPluralName      = BackupCmdName + "s"
	MetadataCmdName          = "metadata"

	pagesFlag    = "pages"
	pagesDefault = 1
	pagesUsage   = "Number of Pages to display within the paginated result set"

	PageSizeFlag = "page-size"

	pageSizeDefault = 10
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
)

var (
	tvkInstanceUID  string
	backupPlanUID   string
	backupStatus    string
	backupUID       string
	creationDate    string
	expiryDate      string
	orderBy         string
	pages, pageSize int
)
