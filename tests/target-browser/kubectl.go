package targetbrowser

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type kubectl struct {
	kubeConfig string
}

func (c *kubectl) createSecretFromFile(namespace, secretName, filename string) error {
	cmd := fmt.Sprintf("kubectl create secret generic %s --from-file=%s=%s %s %s",
		secretName, secretName, filename, c.configArg(), namespaceArg(namespace))
	log.Infof("Creating secret:[%s]", cmd)
	cmdOutStruct, err := RunCmd(cmd)
	if err != nil {
		log.Errorf("(FAILED) Executing kubectl: %s (err: %v): %s", cmd, err, cmdOutStruct.Out)
		return err
	}
	return nil
}

// applyContents applies the given config contents using kubectl.
func (c *kubectl) applyContents(namespace, contents string) ([]string, error) {
	files, err := c.contentsToFileList(contents, "accessor_applyc")
	if err != nil {
		return nil, err
	}

	if err := c.applyInternal(namespace, files); err != nil {
		return nil, err
	}

	return files, nil
}

// apply the config in the given filename using kubectl.
func (c *kubectl) apply(namespace, filename string) error {
	files, err := c.fileToFileList(filename)
	if err != nil {
		return err
	}

	return c.applyInternal(namespace, files)
}

func (c *kubectl) applyInternal(namespace string, files []string) error {
	for _, f := range files {
		command := fmt.Sprintf("kubectl apply %s %s -f %s", c.configArg(), namespaceArg(namespace), f)
		log.Infof("Applying YAML: %s", command)
		s, err := RunCmd(command)
		if err != nil {
			log.Infof("(FAILED) Executing kubectl: %s (err: %v): %s", command, err, s.Out)
			return fmt.Errorf("%v: %s", err, s.Out)
		}
	}
	return nil
}

// deleteContents deletes the given config contents using kubectl.
func (c *kubectl) deleteContents(namespace, contents string) error {
	files, err := c.contentsToFileList(contents, "accessor_deletec")
	if err != nil {
		return err
	}

	return c.deleteInternal(namespace, files)
}

// delete the config in the given filename using kubectl.
func (c *kubectl) delete(namespace, filename string) error {
	files, err := c.fileToFileList(filename)
	if err != nil {
		return err
	}

	return c.deleteInternal(namespace, files)
}

func (c *kubectl) deleteInternal(namespace string, files []string) (err error) {
	for i := len(files) - 1; i >= 0; i-- {
		log.Infof("Deleting YAML file: %s", files[i])
		s, e := Execute(nil, true, "kubectl delete --ignore-not-found %s %s -f %s", c.configArg(), namespaceArg(namespace), files[i])
		if e != nil {
			return multierror.Append(err, fmt.Errorf("%v: %s", e, s.Out))
		}
	}
	return
}

// logs calls the logs command for the specified pod, with -c, if container is specified.
func (c *kubectl) logs(namespace, pod, container string, previousLog bool) (string, error) {
	cmd := fmt.Sprintf("kubectl logs %s %s %s %s %s",
		c.configArg(), namespaceArg(namespace), pod, containerArg(container), previousLogArg(previousLog))

	s, err := Execute(nil, true, cmd)

	if err == nil {
		return s.Out, nil
	}

	return "", fmt.Errorf("%v: %s", err, s.Out)
}

// get calls the kubectl get command for the specified object within namespace.
func (c *kubectl) get(namespace, object string) (string, error) {
	cmd := fmt.Sprintf("kubectl get %s %s %s",
		c.configArg(), namespaceArg(namespace), object)

	s, err := Execute(nil, true, cmd)

	if err == nil {
		return s.Out, nil
	}

	return "", fmt.Errorf("%v: %s", err, s.Out)
}

func (c *kubectl) exec(namespace, pod, container, command string) (string, error) {
	// Don't use combined output. The stderr and stdout streams are updated asynchronously and stderr can
	// corrupt the JSON output.
	s, err := Execute(nil, false, "kubectl exec %s %s %s %s -- %s ", pod,
		namespaceArg(namespace), containerArg(container), c.configArg(), command)
	if err == nil {
		return s.Out, nil
	}

	return "", fmt.Errorf("%v: %s", err, s.Out)
}

func (c *kubectl) cp(namespace, podName, containerName, srcPath, destPath string) error {
	cmd := fmt.Sprintf("kubectl cp %s %s/%s:%s -c %s", srcPath, namespace, podName, destPath, containerName)
	log.Infof("running kubectl cp command %s", cmd)
	return RunCmdWithOutput(cmd)
}

func (c *kubectl) configArg() string {
	return configArg(c.kubeConfig)
}

func (c *kubectl) fileToFileList(filename string) ([]string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	files, err := c.splitContentsToFiles(string(content), filenameWithoutExtension(filename))
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		files = append(files, filename)
	}

	return files, nil
}

func (c *kubectl) contentsToFileList(contents, filenamePrefix string) ([]string, error) {
	files, err := c.splitContentsToFiles(contents, filenamePrefix)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		f, err := c.writeContentsToTempFile(contents)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

func (c *kubectl) writeContentsToTempFile(contents string) (filename string, err error) {
	defer func() {
		if err != nil && filename != "" {
			_ = os.Remove(filename)
			filename = ""
		}
	}()

	workdir := ""

	var f *os.File
	f, err = ioutil.TempFile(workdir, "accessor_")
	if err != nil {
		return
	}
	filename = f.Name()

	_, err = f.WriteString(contents)
	return
}

func (c *kubectl) splitContentsToFiles(content, filenamePrefix string) ([]string, error) {
	cfgs := SplitString(content)

	namespacesAndCrds := &yamlDoc{
		docType: namespacesAndCRDs,
	}
	misc := &yamlDoc{
		docType: misc,
	}
	for _, cfg := range cfgs {
		var typeMeta metav1.TypeMeta
		if e := yaml.Unmarshal([]byte(cfg), &typeMeta); e != nil {
			// Ignore invalid parts. This most commonly happens when it's empty or contains only comments.
			continue
		}

		switch typeMeta.Kind {
		case "Namespace":
			namespacesAndCrds.append(cfg)
		case "CustomResourceDefinition":
			namespacesAndCrds.append(cfg)
		default:
			misc.append(cfg)
		}
	}

	// If all elements were put into a single doc just return an empty list, indicating that the original
	// content should be used.
	docs := []*yamlDoc{namespacesAndCrds, misc}
	for _, doc := range docs {
		if doc.content == "" {
			return make([]string, 0), nil
		}
	}

	filesToApply := make([]string, 0, len(docs))
	for _, doc := range docs {
		workdir := ""

		tfile, err := doc.toTempFile(workdir, filenamePrefix)
		if err != nil {
			return nil, err
		}
		filesToApply = append(filesToApply, tfile)
	}
	return filesToApply, nil
}

func configArg(kubeConfig string) string {
	if kubeConfig != "" {
		return fmt.Sprintf("--kubeconfig=%s", kubeConfig)
	}
	return ""
}

func namespaceArg(namespace string) string {
	if namespace != "" {
		return fmt.Sprintf("-n %s", namespace)
	}
	return ""
}

func containerArg(container string) string {
	if container != "" {
		return fmt.Sprintf("-c %s", container)
	}
	return ""
}

func previousLogArg(previous bool) string {
	if previous {
		return "-p"
	}
	return ""
}

func filenameWithoutExtension(fullPath string) string {
	_, f := filepath.Split(fullPath)
	return strings.TrimSuffix(f, filepath.Ext(fullPath))
}

type docType string

const (
	namespacesAndCRDs docType = "namespaces_and_crds"
	misc              docType = "misc"
)

type yamlDoc struct {
	content string
	docType docType
}

func (d *yamlDoc) append(c string) {
	d.content = JoinString(d.content, c)
}

func (d *yamlDoc) toTempFile(workDir, fileNamePrefix string) (string, error) {
	f, err := ioutil.TempFile(workDir, fmt.Sprintf("%s_%s.yaml", fileNamePrefix, d.docType))
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	name := f.Name()

	_, err = f.WriteString(d.content)
	if err != nil {
		return "", err
	}
	return name, nil
}
