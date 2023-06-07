package cluster

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
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

func checkIfKindInstalled() (bool, error) {
	cmd := exec.Command("command", "-v", "kind")
	err := cmd.Run()
	if err != nil {
		return false, fmt.Errorf("failed to check kind is installed: %v", err)
	}
	return true, nil
}

func installKind() error {
	cmd := exec.Command("go", "install", "sigs.k8s.io/kind@v0.19.0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install kind: %v", err)
	}
	return nil
}

func checkKindInstanceExists() (bool, error) {
	cmd := exec.Command("kind", "get", "clusters")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check kind instance exists: %v", err)
	}
	if strings.Contains(string(out), clusterName) {
		return true, nil
	}
	return false, nil
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
  - containerPort: 9080
    hostPort: 9080
    protocol: TCP
  - containerPort: 9443
    hostPort: 9443
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
            - --enable-ssl-passthrough
            - --http-port=9080
            - --https-port=9443`
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
		return fmt.Errorf("failed to run kubectl patch deployment command: %v", err)
	}

	// patch the deployment ports
	patch := `[{"op":"replace","path":"/spec/template/spec/containers/0/ports/0/containerPort","value":9080},
	{"op": "replace", "path": "/spec/template/spec/containers/0/ports/0/hostPort", "value": 9080},
	{"op": "replace", "path": "/spec/template/spec/containers/0/ports/1/containerPort", "value": 9443},
	{"op": "replace", "path": "/spec/template/spec/containers/0/ports/1/hostPort", "value": 9443}]`
	if err := patchServiceWithJSONPatch("deployment/ingress-nginx-controller", patch); err != nil {
		return fmt.Errorf("failed to run kubectl patch command: %v", err)
	}

	// patch the service ports
	patch = `[{"op": "replace", "path": "/spec/ports/0/port", "value": 9080},
	{"op": "replace", "path": "/spec/ports/1/port", "value": 9443}]`
	if err := patchServiceWithJSONPatch("svc/ingress-nginx-controller", patch); err != nil {
		return fmt.Errorf("failed to run kubectl patch command: %v", err)
	}

	return nil
}

func patchServiceWithJSONPatch(resource, patch string) error {
	cmd := exec.Command("kubectl", "-n", "ingress-nginx", "patch", resource,
		"--type", "json", fmt.Sprintf("-p=%s", patch))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run kubectl patch command: %v", err)
	}
	return nil
}

func CreateKindCluster() {
	done := make(chan bool)
	var wg sync.WaitGroup

	util.PrintStatus("Checking if kind is installed...", done, &wg)
	ok, err := checkIfKindInstalled()
	if err != nil {
		log.Fatalf("Error checking if kind is installed: %v\n", err)
	}
	done <- true

	if !ok {
		util.PrintStatus("Installing kind...", done, &wg)
		err = installKind()
		if err != nil {
			log.Fatalf("Error installing kind: %v\n", err)
		}
		done <- true
	}

	util.PrintStatus("Checking if a kubeflex kind instance already exists...", done, &wg)
	ok, err = checkKindInstanceExists()
	if err != nil {
		log.Fatalf("Error checking if kind instance already exists: %v\n", err)
	}
	done <- true

	if !ok {
		util.PrintStatus("Creating kind cluster...", done, &wg)
		done <- true

		err = createKindInstance(clusterName)
		if err != nil {
			log.Fatalf("Error creating kind instance: %v\n", err)
		}
	}

	util.PrintStatus("Installing and patching nginx ingress...", done, &wg)
	err = installAndPatchNginxIngress()
	if err != nil {
		log.Fatalf("Error installing and patching nginx ingress: %v\n", err)
	}
	done <- true
	wg.Wait()
}