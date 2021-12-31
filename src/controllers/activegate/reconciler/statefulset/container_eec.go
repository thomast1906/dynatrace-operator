package statefulset

import (
	"fmt"
	"regexp"

	"github.com/Dynatrace/dynatrace-operator/src/controllers/activegate/internal/consts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const eecIngestPortName = "eec-http"
const eecIngestPort = 14599
const extensionsLogsDir = "/var/lib/dynatrace/remotepluginmodule/log/extensions"

const activeGateInternalCommunicationPort = 9999

var _ ContainerBuilder = (*ExtensionController)(nil)

type ExtensionController struct {
	GenericContainer
}

func NewExtensionController(stsProperties *statefulSetProperties) *ExtensionController {
	return &ExtensionController{
		GenericContainer: *NewGenericContainer(stsProperties),
	}
}

func (eec *ExtensionController) BuildContainer() corev1.Container {
	return corev1.Container{
		Name:            consts.EecContainerName,
		Image:           eec.image(),
		ImagePullPolicy: corev1.PullAlways,
		Env:             eec.buildEnvs(),
		VolumeMounts:    eec.buildVolumeMounts(),
		Command:         eec.buildCommand(),
		Ports:           eec.buildPorts(),
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.IntOrString{IntVal: eecIngestPort},
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       15,
			FailureThreshold:    3,
			SuccessThreshold:    1,
			TimeoutSeconds:      1,
		},
	}
}

func (eec *ExtensionController) BuildVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "auth-tokens",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "eec-ds-shared",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "dsauthtokendir",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "extensions-logs",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "eec-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: BuildEecConfigMapName(eec.StsProperties.Name, eec.StsProperties.feature),
					},
				},
			},
		},
	}
}

func BuildEecConfigMapName(instanceName string, module string) string {
	if len(instanceName) == 0 || len(module) == 0 {
		err := fmt.Errorf("empty instance or module name not allowed (instance: %s, module: %s)", instanceName, module)
		log.Error(err, "problem building EEC config map name")
		return ""
	}
	return regexp.MustCompile(`[^\w\-]`).ReplaceAllString(instanceName+"-"+module+"-eec-config", "_")
}

func (eec *ExtensionController) image() string {
	if eec.StsProperties.DynaKube.FeatureUseActiveGateImageForStatsD() {
		return eec.StsProperties.DynaKube.ActiveGateImage()
	}
	return eec.StsProperties.DynaKube.EECImage()
}

func (eec *ExtensionController) buildPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: eecIngestPortName, ContainerPort: eecIngestPort},
	}
}

func (eec *ExtensionController) buildCommand() []string {
	if eec.StsProperties.DynaKube.FeatureUseActiveGateImageForStatsD() {
		return []string{
			"/bin/bash", "/dt/eec/entrypoint.sh",
		}
	}
	return nil
}

func (eec *ExtensionController) buildVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: "auth-tokens", MountPath: activeGateConfigDir},
		{Name: "eec-ds-shared", MountPath: "/mnt/dsexecargs"},
		{Name: "dsauthtokendir", MountPath: "/var/lib/dynatrace/remotepluginmodule/agent/runtime/datasources"},
		{Name: "ds-metadata", MountPath: "/opt/dynatrace/remotepluginmodule/agent/datasources/statsd", ReadOnly: true},
		{Name: "extensions-logs", MountPath: extensionsLogsDir},
		{Name: "statsd-logs", MountPath: statsDLogsDir, ReadOnly: true},
		{Name: "eec-config", MountPath: "/var/lib/dynatrace/remotepluginmodule/agent/conf/runtime"},
	}
}

func (eec *ExtensionController) buildEnvs() []corev1.EnvVar {
	tenantId, err := eec.StsProperties.TenantUUID()
	if err != nil {
		eec.StsProperties.log.Error(err, "Problem getting tenant id from api url")
	}
	return []corev1.EnvVar{
		{Name: "TenantId", Value: tenantId},
		{Name: "ServerUrl", Value: fmt.Sprintf("https://localhost:%d/communication", activeGateInternalCommunicationPort)},
		{Name: "EecIngestPort", Value: fmt.Sprintf("%d", eecIngestPort)},
	}
}
