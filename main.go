package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"
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
	writeResults(results, "API Responsive", controlPlaneBool, controlPlaneInfo)
	// Check infrastructure pods health
	infraPodsBool, infraPodInfo := checkInfraHealth(clientset)
	writeResults(results, "Infrastructure Pods Health", infraPodsBool, infraPodInfo)

	// checkNodeAvailability(clientset)
	nodesHealthBool, nodesInfo := checkNodes(clientset)
	writeResults(results, "Node Healthchecks", nodesHealthBool, nodesInfo)

	overCommitBool, overCommitInfo := checkOverCommit(clientset)
	writeResults(results, "Node Overcommit", overCommitBool, overCommitInfo)

	// checkWebhooks
	webhooksBool, webhooksInfo := checkWebhooks(clientset)
	writeResults(results, "Webhooks", webhooksBool, webhooksInfo)

	//checkEndpoints
	endpointsBool, endpointsInfo := checkEndpoints(clientset)
	writeResults(results, "Endpoints", endpointsBool, endpointsInfo)

	// checkEvents
	eventsBool, eventsInfo := checkEvents(clientset)
	writeResults(results, "Events", eventsBool, eventsInfo)

}

func checkOverCommit(clientset *kubernetes.Clientset) (bool, string) {
	info := ""
	ctx := context.Background()
	nodes, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		return false, err.Error()
	}
	for _, n := range nodes.Items {
		cpuAlloc := n.Status.Allocatable.Cpu()
		memAlloc := n.Status.Allocatable.Memory()
		var cpuLimits *resource.Quantity = &resource.Quantity{}
		var memLimits *resource.Quantity = &resource.Quantity{}

		// Find all pods on node n
		podsList, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{FieldSelector: "spec.nodeName=" + n.Name})
		if !check(err) {
			return false, "Failure to get Pod List"
		}
		// For each pod calculate the resource requests and add them to total request
		for _, pod := range podsList.Items {
			for _, container := range pod.Spec.Containers {
				cpuLimits.Add(container.Resources.Limits.Cpu().DeepCopy())
				memLimits.Add(container.Resources.Limits.Memory().DeepCopy())
			}
		}
		// compare requests to allocatable
		// if requests are higher than allocatable set info return false
		if cpuLimits.Value() > cpuAlloc.Value() {
			info += fmt.Sprintf("node %s is overcommited on CPU! Requested: %s Allocateable: %s \n", n.Name, cpuLimits, cpuAlloc)
		}
		if memLimits.Value() > memAlloc.Value() {
			info += fmt.Sprintf("node %s is overcommited on Memory! Requested: %s Allocateable: %s\n", n.Name, memLimits, memAlloc)
		}
	}
	//Nothing overcommited, do not set info, return true
	if info == "" {
		return true, ""
	}
	return false, info
}
func checkEndpoints(clientset *kubernetes.Clientset) (bool, string) {
	info := ""
	ctx := context.Background()

	endpoints, err := clientset.CoreV1().Endpoints("").List(ctx, v1.ListOptions{})
	check(err)

	for _, e := range endpoints.Items {
		if len(e.Subsets) < 1 {
			info = info + fmt.Sprintf("Service %s has no active endpoints!\n", e.Name)
		}
	}

	if info == "" {
		return true, ""
	}
	return false, info

}

func checkWebhooks(clientset *kubernetes.Clientset) (bool, string) {
	info := ""
	ctx := context.Background()

	mutateOutput, MutateErr := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, v1.ListOptions{})
	check(MutateErr)
	validatingOutput, ValidateErr := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, v1.ListOptions{})
	check(ValidateErr)
	for _, mutWebhooks := range mutateOutput.Items {
		for _, webhook := range mutWebhooks.Webhooks {
			if *webhook.FailurePolicy == "Fail" {
				info += fmt.Sprintf("Mutating Webhook: %s has a failurePolicy set to 'Fail'.\n", webhook.Name)
			}
		}
	}
	for _, valWebhooks := range validatingOutput.Items {
		for _, webhook := range valWebhooks.Webhooks {
			if *webhook.FailurePolicy == "Fail" {
				info += fmt.Sprintf("Validating Webhook: %s has a failurePolicy set to 'Fail'.\n", webhook.Name)
			}
		}
	}
	if info == "" {
		return true, ""
	}
	return false, info

}

func checkEvents(clientset *kubernetes.Clientset) (bool, string) {
	info := ""
	ctx := context.Background()

	output, err := clientset.CoreV1().Events("").List(ctx, v1.ListOptions{})
	check(err)

	for _, event := range output.Items {
		if event.Type == "Warning" {
			info += fmt.Sprintf("%s %s/%s %s %s\n", event.Namespace, event.InvolvedObject.Kind, event.InvolvedObject.Name, event.Type, event.Message)
		}
	}
	if info == "" {
		return true, ""
	}
	return false, info
}

// Check for unhealthy nodes
func checkNodes(clientset *kubernetes.Clientset) (bool, string) {
	ctx := context.Background()
	info := ""
	output, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	check(err)

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
	ctx := context.Background()
	output, err := clientset.CoreV1().Pods("kube-system").List(ctx, v1.ListOptions{})
	check(err)
	var info string

	info = ""

	for _, pod := range output.Items {
		for _, container := range pod.Status.ContainerStatuses {

			if container.RestartCount > 0 {
				//if info == "" {
				//	info = info + "\n"
				//}
				info = info + fmt.Sprintf("Container restarts Detected! Pod: %s  container: %s\n", pod.GetName(), container.Name)
			}
			if !container.Ready {
				info = info + fmt.Sprintf("Container 'Not Ready' Detected! Pod: %s  in container: %s\n", pod.GetName(), container.Name)
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
	ctx := context.Background()

	_, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{})
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

func writeResults(buffer *bufio.Writer, component string, result bool, info string) bool {
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
		buffer.Write([]byte(fmt.Sprintf("%s - %s\n%s", symbol, component, info)))
	} else {
		buffer.Write([]byte(fmt.Sprintf("%s - %s\n", symbol, component)))
	}
	err := buffer.Flush()
	return check(err)
}
