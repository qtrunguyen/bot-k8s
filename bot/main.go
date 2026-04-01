package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Event struct {
	Namespace string
	PodName   string
	JobID     string // idempotency key: stable per pod (Kubernetes UID)
}

func main() {
	ctx := context.Background()

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}

	clientset, _ := kubernetes.NewForConfig(config)

	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	for {
		pods, _ := clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{})

		for _, pod := range pods.Items {
			event := Event{
				Namespace: pod.Namespace,
				PodName:   pod.Name,
				JobID:     string(pod.UID),
			}

			data, _ := json.Marshal(event)

			rdb.LPush(ctx, "job-queue", data)

			// Simulate CNI interaction (Cilium-style)
			patch := []byte(`{"metadata":{"annotations":{"cilium.io/policy":"observed"}}}`)
			clientset.CoreV1().Pods(pod.Namespace).
				Patch(ctx, pod.Name, "merge", patch, metav1.PatchOptions{})

			fmt.Println("Queued:", pod.Name)
		}

		time.Sleep(15 * time.Second)
	}
}