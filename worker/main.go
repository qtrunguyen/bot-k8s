package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	queuePending    = "job-queue"
	queueProcessing = "job-processing"
)

type Event struct {
	Namespace string
	PodName   string
	JobID     string // idempotency key (pod UID from bot)
}

func main() {
	ctx := context.Background()

	config, _ := rest.InClusterConfig()
	clientset, _ := kubernetes.NewForConfig(config)

	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	for {
		raw, err := rdb.BRPopLPush(ctx, queuePending, queueProcessing, 0).Result()
		if err != nil {
			fmt.Println("BRPopLPush:", err)
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			fmt.Println("unmarshal:", err)
			if err := rdb.LRem(ctx, queueProcessing, 1, raw).Err(); err != nil {
				fmt.Println("ack poison LRem:", err)
			}
			continue
		}

		if err := applyK8s(ctx, clientset, &event); err != nil {
			fmt.Println("applyK8s:", err)
			continue
		}

		if err := rdb.LRem(ctx, queueProcessing, 1, raw).Err(); err != nil {
			fmt.Println("ack LRem:", err)
		}
	}
}

func applyK8s(ctx context.Context, clientset *kubernetes.Clientset, event *Event) error {
	jobID := event.JobID
	if jobID == "" {
		jobID = event.PodName
	}
	jobName := "mock-" + jobID

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

	_, err := clientset.BatchV1().Jobs("default").Create(ctx, job, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create Job: %w", err)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
		},
		Spec: corev1.ServiceSpec{
			Type: "LoadBalancer",
			Selector: map[string]string{
				"batch.kubernetes.io/job-name": jobName,
			},
			Ports: []corev1.ServicePort{
				{Port: 80},
			},
		},
	}

	_, err = clientset.CoreV1().Services("default").Create(ctx, svc, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create Service: %w", err)
	}

	fmt.Println("Processed:", jobName)
	return nil
}

