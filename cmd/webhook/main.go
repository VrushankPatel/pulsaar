package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = v1.AddToScheme(runtimeScheme)
}

func main() {
	http.HandleFunc("/mutate", handleMutate)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	http.Handle("/metrics", promhttp.Handler())

	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_KEY_FILE")

	if certFile == "" || keyFile == "" {
		log.Fatal("TLS_CERT_FILE and TLS_KEY_FILE must be set")
	}

	server := &http.Server{
		Addr: ":8443",
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	log.Printf("Starting webhook server on :8443")
	log.Fatal(server.ListenAndServeTLS(certFile, keyFile))
}

func handleMutate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var admissionReview v1.AdmissionReview
	if _, _, err := deserializer.Decode(body, nil, &admissionReview); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode: %v", err), http.StatusBadRequest)
		return
	}

	response := &v1.AdmissionResponse{
		UID: admissionReview.Request.UID,
	}

	if admissionReview.Request.Kind.Kind == "Pod" {
		pod := &corev1.Pod{}
		if err := json.Unmarshal(admissionReview.Request.Object.Raw, pod); err != nil {
			response.Result = &metav1.Status{
				Message: err.Error(),
			}
		} else {
			patch, err := mutatePod(pod)
			if err != nil {
				response.Result = &metav1.Status{
					Message: err.Error(),
				}
			} else if patch != nil {
				response.Patch = patch
				response.PatchType = &[]v1.PatchType{v1.PatchTypeJSONPatch}[0]
				response.Allowed = true
			} else {
				response.Allowed = true
			}
		}
	} else {
		response.Allowed = true
	}

	admissionReview.Response = response

	respBytes, err := json.Marshal(admissionReview)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(respBytes)
}

func mutatePod(pod *corev1.Pod) ([]byte, error) {
	// Check for annotation to enable injection
	if pod.Annotations["pulsaar.io/inject-agent"] != "true" {
		return nil, nil
	}

	// Inject sidecar container
	image := os.Getenv("PULSAAR_AGENT_IMAGE")
	if image == "" {
		image = "pulsaar/agent:latest"
	}
	sidecar := corev1.Container{
		Name:  "pulsaar-agent",
		Image: image,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 50051,
				Name:          "grpc",
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "PULSAAR_TLS_CERT_FILE",
				Value: "/etc/pulsaar/tls/tls.crt",
			},
			{
				Name:  "PULSAAR_TLS_KEY_FILE",
				Value: "/etc/pulsaar/tls/tls.key",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "pulsaar-tls",
				MountPath: "/etc/pulsaar/tls",
				ReadOnly:  true,
			},
		},
	}

	pod.Spec.Containers = append(pod.Spec.Containers, sidecar)

	// Inject volume for TLS certs
	volume := corev1.Volume{
		Name: "pulsaar-tls",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "pulsaar-tls",
			},
		},
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, volume)

	// Create JSON patch
	patch := []map[string]interface{}{
		{
			"op":    "add",
			"path":  "/spec/containers/-",
			"value": sidecar,
		},
		{
			"op":    "add",
			"path":  "/spec/volumes/-",
			"value": volume,
		},
	}

	return json.Marshal(patch)
}
