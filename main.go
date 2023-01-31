package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Check *func(*kubernetes.Clientset) *Result

type Result struct {
	// Name The name of the test
	Name string
	// Pass The boolean representing whether the check passed(true)
	Pass bool
	// Details Additional details typically describing an error
	Details string
	// Err any error returned by the check
	Err error
}

/*
Ideas:
- interactive walkthrough of steps with questions for debugging
with user input
- Automatically debug and print out a list of potential checks
- The option to backup and attempt autofix
- Modular debug input files for customization of the tool
*/

func main() {

	clientSet := auth()

	// TODO Allow writing to file at some point
	output := bufio.NewWriter(os.Stdout)

	var resultList []*Result
	var wg sync.WaitGroup

	for _, check := range checks {
		fn := *check
		wg.Add(1)
		go func() {
			resultList = append(resultList, fn(clientSet))
			wg.Done()
		}()
	}
	wg.Wait()

	for _, result := range resultList {
		writeResults(output, result)
	}
}

// authKubeConfig set up a clientset using kubeconfig provided or the default ~/.kube/config
// Returns an authenticated clientset
func authKubeConfig(kubeconfig *string) (*kubernetes.Clientset, error) {

	// Quiet the errors printed to stdOut from BuildConfigFromFlags and NewForConfig
	// commend these two lines out for debugging
	stdErrBackup := os.Stderr
	defer func() {
		os.Stderr = stdErrBackup
	}()

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
	return clientset, nil
}

/*
	Write the results of the given tests to the buffer. This function takes in information

about a test and its results

buffer - A writeBuffer to a file that is where results will be written.
component - The name of the test that the 'result' pertains to.
result - The result from the 'component' test.
info - A string that contains the details of a failure, or "" for a passed test

returns bool for whether the write to file succeeded
*/
func writeResults(buffer *bufio.Writer, r *Result) bool {
	// symbol  ✓
	// symbol  ✗
	colorReset := "\033[0m"
	colorGreen := "\033[32m"
	colorRed := "\033[31m"

	symbol := fmt.Sprintf("%s%s%s", string(colorGreen), "✓", string(colorReset))
	if !r.Pass {
		symbol = fmt.Sprintf("%s%s%s", string(colorRed), "✗", string(colorReset))
	}
	if r.Details != "" {
		_, err := buffer.Write([]byte(fmt.Sprintf("%s - %s\n%s", symbol, r.Name, r.Details)))
		if err != nil {
			return false
		}
	} else {
		_, err := buffer.Write([]byte(fmt.Sprintf("%s - %s\n", symbol, r.Name)))
		if err != nil {
			return false
		}
	}
	err := buffer.Flush()
	if err != nil {
		fmt.Println("Failed flushing buffer for report" + err.Error())
		return false
	}
	return true
}

func auth() *kubernetes.Clientset {
	var kubeconfig *string
	// If home directory is set, have ~/.kube/config be the default
	// TODO do we even need this if, just set as default regardless?
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "(optional) absolute path to the kubeconfig file")
	}
	flag.Parse()

	// Setup auth for cluster
	cs, err := authKubeConfig(kubeconfig)
	if err != nil {
		panic(err)
	}
	return cs
}
