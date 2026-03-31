package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Event struct {
	Namespace string
	PodName   string
}

func main() {
	ctx := context.Background()

	config, _ := rest.InClusterConfig()
	clientset, _ := kubernetes.NewForConfig(config)

	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	for {
		res, _ := rdb.BRPop(ctx, 0, "job-queue").Result()

		var event Event
		json.Unmarshal([]byte(res[1]), &event)

		jobName := "mock-" + event.PodName

		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: jobName,
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: "Never",
						Containers: []corev1.Container{
							{
								Name:    "worker",
								Image:   "busybox",
								Command: []string{"sh", "-c", "echo working && sleep 5"},
							},
						},
					},
				},
			},
		}

		clientset.BatchV1().Jobs("default").Create(ctx, job, metav1.CreateOptions{})

		// Simulate LB controller
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: jobName,
			},
			Spec: corev1.ServiceSpec{
				Type: "LoadBalancer",
				Selector: map[string]string{
					"job-name": jobName,
				},
				Ports: []corev1.ServicePort{
					{Port: 80},
				},
			},
		}

		clientset.CoreV1().Services("default").Create(ctx, svc, metav1.CreateOptions{})

		fmt.Println("Processed:", jobName)
	}
}