package injector

import (
	dtwebhook "github.com/Dynatrace/dynatrace-operator/src/webhook"
	corev1 "k8s.io/api/core/v1"
	"strconv"
	"strings"
)

const (
	initContainerCommand        = "/usr/bin/env"
	initContainerArgBash        = "bash"
	initContainerArgMountScript = "/mnt/config/init.sh"

	envVarNameContainersCount = "CONTAINERS_COUNT"
	envVarNameFailurePolicy   = "FAILURE_POLICY"
	envVarNamePodName         = "K8S_PODNAME"
	envVarNamePodUid          = "K8S_PODUID"
	envVarNameBasePodName     = "K8S_BASEPODNAME"
	envVarNameNamespace       = "K8S_NAMEPSACE"
	envVarNameNodeName        = "K8S_NODE_NAME"

	envVarKeyPodName   = "metadata.name"
	envVarKeyPodUid    = "metadata.uid"
	envVarKeyNamespace = "metadata.namespace"
	envVarKeyNodeName  = "spec.nodeName"
)

func (injector *podInjector) injectInitContainer() {
	pod := injector.pod
	image := injector.image
	failurePolicy := injector.failurePolicy
	basePodName := injector.getBasePodName()
	securityContext := injector.getSecurityContext()
	containerCount := strconv.Itoa(len(pod.Spec.Containers))

	initContainer := corev1.Container{
		Name:            dtwebhook.InstallContainerName,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{initContainerCommand},
		Args:            []string{initContainerArgBash, initContainerArgMountScript},
		Env: []corev1.EnvVar{
			{Name: envVarNameContainersCount, Value: containerCount},
			{Name: envVarNameFailurePolicy, Value: failurePolicy},
			{Name: envVarNameBasePodName, Value: basePodName},
			{Name: envVarNamePodName, ValueFrom: fieldEnvVar(envVarKeyPodName)},
			{Name: envVarNamePodUid, ValueFrom: fieldEnvVar(envVarKeyPodUid)},
			{Name: envVarNameNamespace, ValueFrom: fieldEnvVar(envVarNameNamespace)},
			{Name: envVarNameNodeName, ValueFrom: fieldEnvVar(envVarKeyNodeName)},
		},
		SecurityContext: securityContext,
	}
}

func (injector *podInjector) getSecurityContext() *corev1.SecurityContext {
	pod := injector.pod

	if pod.Spec.Containers[0].SecurityContext != nil {
		return pod.Spec.Containers[0].SecurityContext.DeepCopy()
	}
	return &corev1.SecurityContext{}
}

func (injector *podInjector) getBasePodName() string {
	pod := injector.pod
	basePodName := pod.GenerateName
	if basePodName == "" {
		return pod.Name
	}

	// Only include up to the last dash character, exclusive.
	if p := strings.LastIndex(basePodName, "-"); p != -1 {
		basePodName = basePodName[:p]
	}
	return basePodName
}

func fieldEnvVar(key string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: key}}
}
