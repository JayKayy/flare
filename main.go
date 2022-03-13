package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
*/

func main() {

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// TODO Allow writing to file at some point
	results := bufio.NewWriter(os.Stdout)

	// Setup auth for cluster
	clientset, err := auth(kubeconfig)
	if err != nil {
		panic(err)
	}

	// Run tests and write the results to `results`
	// TODO wrap the tests in goroutines
	// TODO write the results in goroutines

	// Test the control plane apiserver responsiveness and write the results to file
	controlPlaneBool, controlPlaneInfo := checkMasterComponents(clientset)
	writeResults(results, "API Responsive", controlPlaneBool, controlPlaneInfo)
	// Test the infrastructure pods for restarts and write the results to file
	infraPodsBool, infraPodInfo := checkInfraHealth(clientset)
	writeResults(results, "Infrastructure Pods Health", infraPodsBool, infraPodInfo)
	// Test the health of the nodes and write the results to file
	nodesHealthBool, nodesInfo := checkNodes(clientset)
	writeResults(results, "Node Healthchecks", nodesHealthBool, nodesInfo)
	// Test whether the nodes are overcommitted and write the results to file
	overCommitBool, overCommitInfo := checkOverCommit(clientset)
	writeResults(results, "Node Overcommit", overCommitBool, overCommitInfo)
	// Test for the presence of webhooks and their failure policies and write the results to file
	webhooksBool, webhooksInfo := checkWebhooks(clientset)
	writeResults(results, "Webhooks", webhooksBool, webhooksInfo)
	// Test for services without endpoints and write the results to file
	endpointsBool, endpointsInfo := checkEndpoints(clientset)
	writeResults(results, "Endpoints", endpointsBool, endpointsInfo)
	// Test for error or warning events and write the results to file
	eventsBool, eventsInfo := checkEvents(clientset)
	writeResults(results, "Events", eventsBool, eventsInfo)

}

/* These check functions accept an authenticated clientset object and look for specific issues
in the cluster. They all follow the same argument and return signatures:

 If there were no issues found the function returns (true, "").
 If the target issues are found they return (false, str), where str is a string containing
 output relevant to the failure.
*/

// Check if nodes are overcommitted on resources
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
		if err != nil {
			return false, "Failure to get Pod List" + err.Error()
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

// Check if any services have no endpoints
func checkEndpoints(clientset *kubernetes.Clientset) (bool, string) {
	info := ""
	ctx := context.Background()

	endpoints, err := clientset.CoreV1().Endpoints("").List(ctx, v1.ListOptions{})
	if err != nil {
		return false, "Failure to get endpoints" + err.Error()
	}
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

// Check if any webhooks are installed with a failure policy of 'Fail'
func checkWebhooks(clientset *kubernetes.Clientset) (bool, string) {
	info := ""
	ctx := context.Background()

	mutateOutput, errMutate := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, v1.ListOptions{})
	if errMutate != nil {
		return false, "Failed getting mutatingwebhooks " + errMutate.Error()
	}
	validatingOutput, errValidate := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, v1.ListOptions{})
	if errValidate != nil {
		return false, "Failed getting validatingwebhooks " + errValidate.Error()
	}
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

// Check if any events are showing warnings
func checkEvents(clientset *kubernetes.Clientset) (bool, string) {
	info := ""
	ctx := context.Background()

	output, err := clientset.CoreV1().Events("").List(ctx, v1.ListOptions{})
	if err != nil {
		return false, "Failed getting events " + err.Error()
	}
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

// Check for nodes in UnReady status
func checkNodes(clientset *kubernetes.Clientset) (bool, string) {
	ctx := context.Background()
	info := ""
	output, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		return false, "failed getting nodes" + err.Error()
	}
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

// Check whether there are pods with restarts in the kube-system namespace
func checkInfraHealth(clientset *kubernetes.Clientset) (bool, string) {
	ctx := context.Background()
	output, err := clientset.CoreV1().Pods("kube-system").List(ctx, v1.ListOptions{})

	if err != nil {
		return false, "failed getting kube-system pods " + err.Error()
	}
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

// Check that the apiserver responds
func checkMasterComponents(clientset *kubernetes.Clientset) (bool, string) {
	ctx := context.Background()

	_, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		return false, "Connectivity failure" + err.Error()
	}
	return true, ""
}

// Setup a clientset using kubeconfig provided or the default ~/.kube/config
// Returns an authenticated clientset
func auth(kubeconfig *string) (*kubernetes.Clientset, error) {

	// Quiet the errors printed to stdOut from BuildConfigFromFlags and NewForConfig
	// commend these two lines out for debugging
	stdErrBackup := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)

	config, errBuildConf := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if errBuildConf != nil {
		//	fmt.Println("Could not build config. Returning err: " + errBuildConf.Error())
		os.Stderr = stdErrBackup
		return nil, errBuildConf
	}
	clientset, errClient := kubernetes.NewForConfig(config)
	if errClient != nil {
		//	fmt.Println("Failed creating clientset. Returning err: " + errClient.Error())
		os.Stderr = stdErrBackup
		return nil, errClient
	}
	os.Stderr = stdErrBackup
	return clientset, nil
}

/* Write the results of the given tests to the buffer. This function takes in information
about a test and its results

buffer - A writeBuffer to a file that is where results will be written.
component - The name of the test that the 'result' pertains to.
result - The result from the 'component' test.
info - A string that contains the details of a failure, or "" for a passed test

returns bool for whether the write to file succeeded
*/
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
	if err != nil {
		fmt.Println("Failed flushing buffer for report" + err.Error())
		return false
	}
	return true
}
