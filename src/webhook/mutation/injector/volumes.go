package injector

import (
	dtcsi "github.com/Dynatrace/dynatrace-operator/src/controllers/csi"
	dtingestendpoint "github.com/Dynatrace/dynatrace-operator/src/ingestendpoint"
	dtwebhook "github.com/Dynatrace/dynatrace-operator/src/webhook"
	corev1 "k8s.io/api/core/v1"
)

const (
	injectionConfigVolumeName = "injection-config"

	oneAgentBinVolumeName   = "oneagent-bin"
	oneAgentShareVolumeName = "oneagent-share"

	dataIngestVolumeName         = "data-ingest-enrichment"
	dataIngestEndpointVolumeName = "data-ingest-endpoint"
)

func (injector *podInjector) injectConfigVolume() {
	pod := injector.pod
	pod.Spec.Volumes = append(pod.Spec.Volumes, createConfigVolume())
}

func (injector *podInjector) injectOneAgentVolumes() {
	pod := injector.pod

	if !injector.injectOneAgent {
		return
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes, injector.createOneAgentVolumes()...)
}

func (injector *podInjector) injectDataIngestVolumes() {
	pod := injector.pod

	if !injector.injectDataIngest {
		return
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes, createDataIngestVolumes()...)
}

func createDataIngestVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: dataIngestVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: dataIngestEndpointVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: dtingestendpoint.SecretEndpointName,
				},
			},
		},
	}
}

func (injector *podInjector) createOneAgentVolumes() []corev1.Volume {
	return []corev1.Volume{
		{Name: oneAgentBinVolumeName, VolumeSource: injector.createOneAgentVolumeSource()},
		{Name: oneAgentShareVolumeName, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}
}

func (injector *podInjector) createOneAgentVolumeSource() corev1.VolumeSource {
	dynakube := injector.dynakube

	if dynakube.NeedsCSIDriver() {
		return corev1.VolumeSource{CSI: &corev1.CSIVolumeSource{Driver: dtcsi.DriverName}}
	}
	return corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
}

func createConfigVolume() corev1.Volume {
	return corev1.Volume{
		Name:         injectionConfigVolumeName,
		VolumeSource: createConfigVolumeSource(),
	}
}

func createConfigVolumeSource() corev1.VolumeSource {
	return corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName: dtwebhook.SecretConfigName,
		},
	}
}
