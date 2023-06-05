package cluster

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"text/template"

	"mcc.ibm.org/kubeflex/pkg/util"
)

const (
	clusterName = "kubeflex"
)

// KindConfig is a struct that represents the kind cluster configuration
type KindConfig struct {
	Name string
}

// createKindInstance creates a kind cluster with the given name and config
func createKindInstance(name string) error {
	// create a template for the kind config file
	tmpl := template.Must(template.New("kind-config").Parse(`kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP`))

	// create a buffer to write the template output
	var buf bytes.Buffer

	// execute the template with the name parameter
	err := tmpl.Execute(&buf, KindConfig{Name: name})
	if err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	// create a temporary file to store the kind config file
	tmpFile, err := os.CreateTemp("", "kind-config-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// write the buffer content to the temp file
	_, err = tmpFile.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	// close the temp file
	err = tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close temp file: %v", err)
	}

	// run the kind create cluster command with the temp file as config
	cmd := exec.Command("kind", "create", "cluster", "--name", name, "--config", tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run kind create cluster command: %v", err)
	}

	return nil
}

// installAndPatchNginxIngress installs and patches the nginx ingress controller on the kind cluster
func installAndPatchNginxIngress() error {
	// run the kubectl apply command to install the nginx ingress controller
	cmd := exec.Command("kubectl", "apply", "-f", "https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run kubectl apply command: %v", err)
	}

	// create a patch file for the nginx controller deployment
	patchFile, err := os.CreateTemp("", "nginx-controller-patch-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create patch file: %v", err)
	}
	defer os.Remove(patchFile.Name())

	// write the patch content to the patch file
	patchContent := `spec:
  template:
    spec:
      containers:
        - name: controller
          args:
            - /nginx-ingress-controller
            - --election-id=ingress-nginx-leader
            - --controller-class=k8s.io/ingress-nginx
            - --ingress-class=nginx
            - --configmap=$(POD_NAMESPACE)/ingress-nginx-controller
            - --validating-webhook=:8443
            - --validating-webhook-certificate=/usr/local/certificates/cert
            - --validating-webhook-key=/usr/local/certificates/key
            - --watch-ingress-without-class=true
            - --publish-status-address=localhost
            - --enable-ssl-passthrough`
	_, err = patchFile.WriteString(patchContent)
	if err != nil {
		return fmt.Errorf("failed to write patch file: %v", err)
	}

	err = patchFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close patch file: %v", err)
	}

	// run the kubectl patch command to patch the nginx controller deployment with the patch file
	cmd = exec.Command("kubectl", "-n", "ingress-nginx", "patch", "deployment/ingress-nginx-controller",
		fmt.Sprintf("--patch-file=%s", patchFile.Name()))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run kubectl patch command: %v", err)
	}

	return nil
}

func CreateKindCluster() {
	done := make(chan bool)
	var wg sync.WaitGroup
	util.PrintStatus("Creating kind cluster...", done, &wg)
	done <- true

	err := createKindInstance(clusterName)
	if err != nil {
		log.Fatalf("Error creating kind instance: %v\n", err)
	}

	util.PrintStatus("Installing and patching nginx ingress...", done, &wg)
	err = installAndPatchNginxIngress()
	if err != nil {
		log.Fatalf("Error installing and patching nginx ingress: %v\n", err)
	}
	done <- true
	wg.Wait()
}
