package main

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMutatePod(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool // true if patch is generated
	}{
		{
			name: "no annotation",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "nginx"},
					},
				},
			},
			expected: false,
		},
		{
			name: "with annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"pulsaar.io/inject-agent": "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "nginx"},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := mutatePod(tt.pod)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if (patch != nil) != tt.expected {
				t.Errorf("expected patch %v, got %v", tt.expected, patch != nil)
			}
			if patch != nil {
				var operations []map[string]interface{}
				if err := json.Unmarshal(patch, &operations); err != nil {
					t.Fatalf("invalid patch: %v", err)
				}
				if len(operations) != 2 {
					t.Errorf("expected 2 operations, got %d", len(operations))
				}
			}
		})
	}
}
