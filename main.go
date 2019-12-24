package main

import (
	"bufio"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

	// Setup auth for cluster
	clientset := auth()

	// Check health of master components
	checkMasterComponents(clientset)

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

func checkMasterComponents(clientset *kubernetes.Clientset) (bool, *string) {
	componentsClient := clientset.AppsV1().ComponentStatus(apiv1.NamespaceDefault)
	fmt.Printf("components status: %s\n", reflect.TypeOf(componentsClient))
}
func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
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
