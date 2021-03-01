package utils

const (
	TestIDLength        = 8
	DataMoverNamespace  = "datamover-tests"
	DataMoverPodBackup  = "datamover-pod-backup"
	DataMoverPodRestore = "datamover-pod-restore"
	DataInsertionPod    = "datainsertion-pod"
	DataModificationPod = "datamodification-pod"
	DataValidationPod   = "datavalidation-pod"
	StorageClassName    = "csi-gcp-sc"
	FileSystemModePV    = "FileSystem"
	BlockModePV         = "Block"
	FirstTimeBackup     = "firsttime"
	IncrementalBackup   = "incremental"
	Qcow2PV             = "pv.qcow2"
	IntermediateQcow2PV = "intermediate.qcow2"
	Qcow2Format         = "qcow2"

	// DataMover images
	AlpineImage = "alpine:latest"

	// integration tests paths
	FsFirstTimeBackupPath    = "bkp-1"
	FsIncrementalBackupPath  = "bkp-2"
	BlkFirstTimeBackupPath   = "blk-bkp-1"
	BlkIncrementalBackupPath = "blk-bkp-2"
	FsMountPathForBackup     = "/sample/data"
	FsMountPathForRestore    = "/restore/data"
	FsRestoreTestDataPath    = "fs-restore-testdata"
	BlkRestoreTestDataPath   = "block-restore-testdata"

	// Integration tests actions
	Backup              = "backup"
	Restore             = "restore"
	FsDataInsertion     = "data-insert-filesystem"
	BlockDataInsertion  = "data-insert-block"
	FsDataValidation    = "data-validate-filesystem"
	BlockDataValidation = "data-validate-block"

	// Data Mover POD Container names
	DataMoverContainer      = "datamover-container"
	SidecarContainer        = "sidecar-container"
	DataInsertionContainer  = "datainsertion-container"
	DataValidationContainer = "datavalidation-container"

	// DataMover yaml paths
	TrilioSecYaml       = "trilio-secret.yaml"
	BlockPvcRestoreYaml = "raw-pv-pvc-restore.yaml"
	BlockPvcBackupYaml  = "raw-pv-sample_pvc.yaml"

	TrilioSecName = "trilio-secret"

	// DataMover storage class
	CSIStorageClassYAML = "csi-storageclass.yaml"

	// DataMover PVC
	FsPersistentVolumeClaim           = "fs-pvc-backup"
	FsPersistentVolumeClaimRestore    = "fs-pvc-restore"
	PersistentVolumeSize              = "100M"
	BlockPersistentVolumeClaim        = "block-pvc"
	BlockPersistentVolume             = "block-pv"
	BlockPersistentVolumeClaimRestore = "block-pvc-restore"
	BlockPersistentVolumeRestore      = "block-pv-restore"

	// DataMover Test Directory names
	BaseDir     = "/tmp"
	LoopDirName = "/LoopDeviceInfo"

	// DataMover Test Image names
	BlankFile             = "blank"
	SrcFile               = "base"
	Overlay               = "overlay"
	RestoreSampleData     = "restoreVirual"
	RestoreSampleDataTest = "restoreVirualTest"
	RestoreData           = "restoreActual"

	// DataMover Test source Inputs
	BlankInputFile      = "/dev/zero"
	RandomDataInputFile = "/dev/urandom"

	BlockSize  = "1M"
	LoopDevice = "/dev/loop"

	// Test Image size consts
	SrcDiskSize             = "10"
	DataStoreSize           = "50"
	SrcDataModificationSize = "4"
	FileSystemDiskSize      = "10"
	RestoreVirtualSize      = "15"
	RestoreDataSize         = "10"
)
