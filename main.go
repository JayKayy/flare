package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

/*
Ideas:
- interactive walkthrough of steps with questions for debugging
with user input
- Automatically debug and print out a list of potential checks
- The option to backup and attempt autofix
- Modular debug input files for customization of the tool


SAMPLE checks:

check for master api connectivity
check for pod restarts
check node availability
check node capacity for workload
check a service has valid endpoints
check pvc's status
check a pv's status
check componentstatus

*/
func main() {

	// Allow writing to file at some point

	results := bufio.NewWriter(os.Stdout)
	// Setup auth for cluster
	clientset := auth()

	// Check health of master components
	controlPlaneBool, controlPlaneInfo := checkMasterComponents(clientset)
	tallyResults(results, "API Responsive", controlPlaneBool, controlPlaneInfo)
	// Check infrastructure pods health
	infraPodsBool, infraPodInfo := checkInfraHealth(clientset)
	tallyResults(results, "Infrastructure Pods Health", infraPodsBool, infraPodInfo)

	nodesBool, nodesInfo := checkNodes(clientset)
	tallyResults(results, "Node Healthchecks", nodesBool, nodesInfo)

	// checkNodeAvailability(clientset)
	// checkNodeCapacity(clientset)
	// checkEvents
	// checkWebhooks
	// check logs of pods
}

// Check for unhealthy nodes
func checkNodes(clientset *kubernetes.Clientset) (bool, string) {
	output, err := clientset.CoreV1().Nodes().List(v1.ListOptions{})
	check(err)
	info := ""

	for _, node := range output.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" {
				if condition.Status == "False" {
					info += fmt.Sprintf("Node: %s is NotReady", node.Name)
				}
			}
		}
	}
	if info != "" {
		return false, info
	}
	return true, ""
}

// Detect whether there are pod restarts in the kube-system namespace
func checkInfraHealth(clientset *kubernetes.Clientset) (bool, string) {
	output, err := clientset.CoreV1().Pods("kube-system").List(v1.ListOptions{})
	check(err)
	var info string

	info = ""

	for _, pod := range output.Items {
		for _, container := range pod.Status.ContainerStatuses {

			if container.RestartCount > 0 {
				//if info == "" {
				//	info = info + "\n"
				//}
				info = info + fmt.Sprintf("%s - %s\n", pod.GetName(), container.Name)
			}
		}
	}
	if info == "" {
		return true, ""
	}
	return false, info
}

// Check that the API responds
func checkMasterComponents(clientset *kubernetes.Clientset) (bool, string) {
	_, err := clientset.CoreV1().Nodes().List(v1.ListOptions{})
	if !check(err) {
		return false, "Connectivity failure"
	}
	return true, ""

}

// Check errors for Fatal
func check(e error) bool {
	if e != nil {
		log.Fatal(e)
		return false
	}
	return true
}

func prompt() {
	fmt.Printf("-> Press Return key to continue.")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		break
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	fmt.Println()
}

func auth() *kubernetes.Clientset {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	check(err)

	clientset, err := kubernetes.NewForConfig(config)
	check(err)

	return clientset
}

func tallyResults(buffer *bufio.Writer, component string, result bool, info string) bool {
	// symbol  ✓
	// symbol  ✗
	colorReset := "\033[0m"
	colorGreen := "\033[32m"
	colorRed := "\033[31m"
	symbol := fmt.Sprintf("%s%s%s", string(colorGreen), "✓", string(colorReset))
	if !result {
		symbol = fmt.Sprintf("%s%s%s", string(colorRed), "✗", string(colorReset))
	}
	if info != "" {
		buffer.Write([]byte(fmt.Sprintf("%s - %s\n %s\n", symbol, component, info)))
	} else {
		buffer.Write([]byte(fmt.Sprintf("%s - %s\n", symbol, component)))
	}
	err := buffer.Flush()
	return check(err)
}
