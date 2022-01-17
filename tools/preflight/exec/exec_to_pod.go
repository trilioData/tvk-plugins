package exec

import (
	"bytes"
	"fmt"
	"io"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	client "k8s.io/client-go/kubernetes"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	tty             = false
	podResource     = "pods"
	execSubresource = "exec"
	POSTMethod      = "POST"
)

// Response contain's command execution response
type Response struct {
	Stdout, Stderr string
	Err            error
}

// Options passed to ExecWithOptions
type Options struct {
	Command []string

	Namespace     string
	PodName       string
	ContainerName string

	Executor  RemoteExecutor
	Config    *restclient.Config
	ClientSet *client.Clientset
}

// RemoteExecutor defines the interface accepted by the Exec command - provided for test stubbing
type RemoteExecutor interface {
	execute(method string, url *url.URL, config *restclient.Config, stdout, stderr io.Writer, tty bool) error
}

// DefaultRemoteExecutor is the standard implementation of remote command execution
type DefaultRemoteExecutor struct{}

func (*DefaultRemoteExecutor) execute(method string, reqURL *url.URL, config *restclient.Config, stdout, stderr io.Writer, tty bool) error {
	exec, err := remotecommand.NewSPDYExecutor(config, method, reqURL)
	if err != nil {
		return err
	}

	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	})
}

// ExecInContainer will execute given command's in specified container and return execution response in channel `resp`.
func (op *Options) ExecInContainer(execChan chan *Response) {
	var cmdOut, errStr string
	var err error

	// exec module should know on which pod & container the commands needs to be executed so if
	// any one of them is not given then exec module can't process further.
	if op.PodName == "" || op.ContainerName == "" {
		execChan <- &Response{
			Stdout: "",
			Stderr: "",
			Err:    fmt.Errorf("pod name or container name is empty"),
		}
	}

	req := op.ClientSet.CoreV1().RESTClient().Post().Resource(podResource).
		Name(op.PodName).Namespace(op.Namespace).
		SubResource(execSubresource)

	req.VersionedParams(&corev1.PodExecOptions{
		Container: op.ContainerName,
		Command:   op.Command,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       tty,
	}, clientGoScheme.ParameterCodec)

	var stdout, stderr bytes.Buffer

	err = op.Executor.execute(POSTMethod, req.URL(), op.Config, &stdout, &stderr, tty)

	cmdOut = stdout.String()
	errStr = stderr.String()

	execChan <- &Response{
		Stdout: cmdOut,
		Stderr: errStr,
		Err:    err,
	}
}
