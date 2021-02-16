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
	controlPlaneHealth := checkMasterComponents(clientset)
	// Write  ✓ Control Plane Health to buffer
	tallyResults(results, "API Responsive", controlPlaneHealth)
	// Check infrastructure pods health
	//checkInfraHealth(clientset)

	// Check Nodes

	//checkNodeAvailability(clientset)
	//checkNodeCapacity(clientset)

	/*	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "demo-deployment",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "demo",
					},
				},
				Template: apiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "demo",
						},
					},
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:  "web",
								Image: "nginx:1.12",
								Ports: []apiv1.ContainerPort{
									{
										Name:          "http",
										Protocol:      apiv1.ProtocolTCP,
										ContainerPort: 80,
									},
								},
							},
						},
					},
				},
			},
		}

		// Create

		fmt.Println("Creating Deployment...")
		result, err := deploymentsClient.Create(deployment)
		if !strings.Contains(err.Error(), "already exists") {
			check(err)
		} else {
			fmt.Println("Object already exists!")
		}
		log.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

		prompt()
		fmt.Println("Updating deployment")

		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			result, getErr := deploymentsClient.Get("demo-deployment", metav1.GetOptions{})
			if getErr != nil {
				check(fmt.Errorf("Failed to get latest version of Deployment: %v", getErr))
			}
			result.Spec.Replicas = int32Ptr(1)
			result.Spec.Template.Spec.Containers[0].Image = "nginx:latest"
			_, updateErr := deploymentsClient.Update(result)
			return updateErr

		})
		check(retryErr)
		fmt.Println("Updated deployment!")
	*/
}

func checkMasterComponents(clientset *kubernetes.Clientset) bool {
	_, err := clientset.CoreV1().Nodes().List(v1.ListOptions{})
	check(err)

	return true
}
func check(e error) bool {
	if e != nil {
		log.Fatal(e)
		return false
	}
	return true
}
func int32Ptr(i int32) *int32 { return &i }
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
func tallyResults(buffer *bufio.Writer, component string, result bool) bool {
	// symbol  ✓
	// symbol  ✗
	colorReset := "\033[0m"
	colorGreen := "\033[32m"
	colorRed := "\033[31m"
	symbol := fmt.Sprintf("%s%s%s", string(colorGreen), "✓", string(colorReset))
	if !result {
		symbol = fmt.Sprintf("%s%s%s", string(colorRed), "✗", string(colorReset))
	}
	buffer.Write([]byte(fmt.Sprintf("%s - %s\n", symbol, component)))
	err := buffer.Flush()
	return check(err)

}
