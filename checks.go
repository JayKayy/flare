package main

import (
	"context"
	"fmt"

	v1batch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

/* These check functions accept an authenticated clientset object and look for specific issues
in the cluster. They all follow the same argument and return signatures:

 If there were no issues found the function returns (true, "").
 If the target issues are found they return (false, str), where str is a string containing
 output relevant to the failure.
*/

var (
	cronJobThreshold = 100
)

// Check that the apiserver responds
var (
	cp = func(clientset *kubernetes.Clientset) *Result {
		ctx := context.Background()

		_, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{})
		if err != nil {

			return &Result{
				Name:    "control plane connectivity",
				Pass:    false,
				Details: "connectivity failure",
				Err:     err,
			}
		}
		return &Result{
			Name: "control plane connectivity",
			Pass: true,
		}
	}

	// Check if nodes are overcommitted on resources
	overCommit = func(clientset *kubernetes.Clientset) *Result {
		name := "overcommit"
		info := ""
		ctx := context.Background()
		nodes, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{})
		if err != nil {
			return &Result{
				Name:    name,
				Pass:    false,
				Details: "node(s) overcommitted",
				Err:     err,
			}
		}
		for _, n := range nodes.Items {
			cpuAlloc := n.Status.Allocatable.Cpu()
			memAlloc := n.Status.Allocatable.Memory()
			var cpuLimits *resource.Quantity = &resource.Quantity{}
			var memLimits *resource.Quantity = &resource.Quantity{}

			// Find all pods on node n
			podsList, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{FieldSelector: "spec.nodeName=" + n.Name})
			if err != nil {
				return &Result{
					Name:    name,
					Pass:    false,
					Details: "node(s) overcommitted",
					Err:     err,
				}
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
			return &Result{
				Name: name,
				Pass: true,
			}
		}
		return &Result{
			Name:    name,
			Pass:    true,
			Details: info,
			Err:     err,
		}
	}

	// Check if any services have no endpoints
	endpoints = func(clientset *kubernetes.Clientset) *Result {
		name := "endpoints"
		info := ""
		ctx := context.Background()

		endpoints, err := clientset.CoreV1().Endpoints("").List(ctx, v1.ListOptions{})
		if err != nil {
			return &Result{
				Name:    name,
				Pass:    false,
				Details: "getting endpoints",
				Err:     err,
			}
		}
		for _, e := range endpoints.Items {
			if len(e.Subsets) < 1 {
				info = info + fmt.Sprintf("Service %s has no active endpoints!\n", e.Name)
			}
		}

		if info == "" {
			return &Result{
				Name: name,
				Pass: true,
			}
		}
		return &Result{
			Name:    name,
			Pass:    false,
			Details: info,
			Err:     err,
		}
	}

	// Check if any webhooks are installed with a failure policy of 'Fail'
	webhooks = func(clientset *kubernetes.Clientset) *Result {
		name := "webhooks"
		info := ""
		ctx := context.Background()

		mutateOutput, errMutate := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, v1.ListOptions{})
		if errMutate != nil {
			return &Result{
				Name:    name,
				Pass:    false,
				Details: "getting mutatingwebhookconfigurations",
				Err:     errMutate,
			}
		}
		validatingOutput, errValidate := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, v1.ListOptions{})
		if errValidate != nil {
			return &Result{
				Name:    name,
				Pass:    false,
				Details: "gettingvalidatingwebhookconfigurations",
				Err:     errValidate,
			}
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
			return &Result{
				Name: name,
				Pass: true,
			}
		}
		return &Result{
			Name:    name,
			Pass:    false,
			Details: info,
			Err:     nil,
		}
	}

	// Check if any events are showing warnings
	events = func(clientset *kubernetes.Clientset) *Result {
		name := "events"
		info := ""
		ctx := context.Background()

		output, err := clientset.CoreV1().Events("").List(ctx, v1.ListOptions{})
		if err != nil {
			return &Result{
				Name:    name,
				Pass:    false,
				Details: err.Error(),
				Err:     err,
			}
		}
		for _, event := range output.Items {
			if event.Type == "Warning" {
				info += fmt.Sprintf("%s %s/%s %s %s\n", event.Namespace, event.InvolvedObject.Kind, event.InvolvedObject.Name, event.Type, event.Message)
			}
		}
		if info == "" {
			return &Result{
				Name: name,
				Pass: true,
			}
		}
		return &Result{
			Name:    name,
			Pass:    false,
			Details: info,
		}
	}

	// Check for nodes in UnReady status
	nodes = func(clientset *kubernetes.Clientset) *Result {
		name := "nodes"
		ctx := context.Background()
		info := ""
		output, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{})
		if err != nil {
			return &Result{
				Name:    name,
				Pass:    false,
				Details: err.Error(),
				Err:     err,
			}
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
			return &Result{
				Name:    name,
				Pass:    false,
				Details: info,
				Err:     err,
			}
		}
		return &Result{
			Name: name,
			Pass: true,
		}
	}

	// Check whether there are pods with restarts in the kube-system namespace
	infra = func(clientset *kubernetes.Clientset) *Result {
		name := "infra"
		ctx := context.Background()
		output, err := clientset.CoreV1().Pods("kube-system").List(ctx, v1.ListOptions{})

		if err != nil {
			return &Result{
				Name:    name,
				Pass:    false,
				Details: "failed to get kube-system pods",
				Err:     err,
			}
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
			return &Result{
				Name: name,
				Pass: true,
			}
		}
		return &Result{
			Name:    name,
			Pass:    false,
			Details: info,
		}
	}
	cronjob = func(clientset *kubernetes.Clientset) *Result {
		name := "cronjobs"
		ctx := context.Background()
		info := ""
		output, err := clientset.BatchV1().CronJobs("").List(ctx, v1.ListOptions{})
		if err != nil {
			info = "Failed to get cronjobs"
			return &Result{
				Name:    name,
				Pass:    false,
				Details: info,
				Err:     err,
			}
		}
		var pass = true
		for _, cron := range output.Items {
			if len(cron.Status.Active) > cronJobThreshold {
				info += fmt.Sprintf("Cronjob %s/%s, has too many active jobs: %d\n", cron.Namespace, cron.Name, len(cron.Status.Active))
				pass = false
			}
			// This may not always be wrong consider a warning status
			if cron.Spec.ConcurrencyPolicy == v1batch.AllowConcurrent {
				info += fmt.Sprintf("⚠️ Cronjob %s, is set to AllowConcurrent\n", cron.Name)
			}
		}
		return &Result{
			Name:    name,
			Pass:    pass,
			Details: info,
			Err:     err,
		}
	}
	oomkilled = func(clientset *kubernetes.Clientset) *Result {
		name := "oomkilled"
		ctx := context.Background()
		info := ""
		output, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{})
		if err != nil {
			info += "Failed to get pods"
			return &Result{
				Name:    name,
				Pass:    false,
				Details: info,
				Err:     err,
			}
		}
		pass := true
		for _, pod := range output.Items {
			for _, status := range pod.Status.ContainerStatuses {
				if status.LastTerminationState.Terminated != nil {
					if status.LastTerminationState.Terminated.Reason == "OOMKilled" {
						pass = false
						info += fmt.Sprintf("Pod %s/%s has previous OOMKilled state\n", pod.GetNamespace(), pod.GetName())
					}
				}
			}
		}
		return &Result{
			Name:    name,
			Pass:    pass,
			Details: info,
			Err:     err,
		}
	}
	// for running singular checks
	//checks = []Check{&oomkilled}
	checks = []Check{&cp, &endpoints, &events, &infra, &nodes, &overCommit, &webhooks, &cronjob, &oomkilled}
)
