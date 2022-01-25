package mutation

import (
	"github.com/Dynatrace/dynatrace-operator/src/kubeobjects"
	dtwebhook "github.com/Dynatrace/dynatrace-operator/src/webhook"
	v1 "k8s.io/api/core/v1"
	"net/url"
)

type basicData struct {
	technologies  string
	installPath   string
	installerURL  string
	failurePolicy string
	image         string
}

func createBasicDataFromPod(pod *v1.Pod, image string) *basicData {
	technologies := url.QueryEscape(kubeobjects.GetField(pod.Annotations, dtwebhook.AnnotationTechnologies, "all"))
	installPath := kubeobjects.GetField(pod.Annotations, dtwebhook.AnnotationInstallPath, dtwebhook.DefaultInstallPath)
	installerURL := kubeobjects.GetField(pod.Annotations, dtwebhook.AnnotationInstallerUrl, "")
	failurePolicy := kubeobjects.GetField(pod.Annotations, dtwebhook.AnnotationFailurePolicy, "silent")
	return &basicData{
		technologies:  technologies,
		installPath:   installPath,
		installerURL:  installerURL,
		failurePolicy: failurePolicy,
		image:         image,
	}
}
