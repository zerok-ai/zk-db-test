package k8s

import (
	"context"
	zkLogger "github.com/zerok-ai/zk-utils-go/logs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const LoggerTag = "k8s"

type PodDetails struct {
	Name string
	IP   string
}

func GetPodNameAndIPs(namespace, labelSelector string) []PodDetails {

	podList, err := getPodsWithLabel(namespace, labelSelector)
	if err != nil {
		zkLogger.ErrorF(LoggerTag, "error in GetPodNameAndIPs %v\n", err)
		return []PodDetails{}
	}

	podNames := make([]PodDetails, 0)
	for _, pod := range podList.Items {
		name := pod.GetName()
		ip := pod.Status.PodIP
		if name == "" || ip == "" {
			continue
		}
		podNames = append(podNames, PodDetails{Name: name, IP: ip})
	}

	return podNames
}

func getPodsWithLabel(namespace, labelSelector string) (*corev1.PodList, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	pods, _ := clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	return pods, nil
}
