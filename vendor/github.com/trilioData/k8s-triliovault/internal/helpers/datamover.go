package helpers

import (
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/trilioData/k8s-triliovault/internal/utils/shell"
	"k8s.io/apimachinery/pkg/util/json"
)

type ImageInfo struct {
	Size        string
	BackingFile string
	ImagePath   string
}

// GetImageInfo parses the specfied qemu-img.
// input=>imagePath: source qemu-img path for parsing.
// output=>imageInfo: contains parsed image info.
// 		   errStr: returns stdout if command successfully executes else return stderr.
// 		   err: non-nil error if command execution failed.
func GetImageInfo(imagePath string) (imageInfo *ImageInfo, errStr string, err error) {
	imageInfo = &ImageInfo{ImagePath: imagePath, Size: "0", BackingFile: ""}
	cmd := fmt.Sprintf("qemu-img info %s --output json", imagePath)
	outStruct, err := shell.RunCmd(cmd)
	if err != nil {
		log.Errorf("Failed:%s", outStruct.Out)
		errStr = outStruct.Out
		return imageInfo, errStr, err
	}
	outputMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(outStruct.Out), &outputMap)
	if err != nil {
		log.Errorf("Failed:[%s]", err.Error())
		errStr = err.Error()
		return imageInfo, errStr, err
	}
	actualSize := outputMap["actual-size"]
	switch size := actualSize.(type) {
	case float64:
		imageInfo.Size = strconv.FormatInt(int64(size), 10)
	case int64:
		imageInfo.Size = strconv.FormatInt(size, 10)
	}
	if _, ok := outputMap["backing-filename"]; ok {
		imageInfo.BackingFile = outputMap["backing-filename"].(string)
	}
	return imageInfo, errStr, err
}
