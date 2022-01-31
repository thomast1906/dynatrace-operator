package injector

import (
	dynatracev1beta1 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

type podInjector struct {
	pod              *corev1.Pod
	dynakube         dynatracev1beta1.DynaKube
	technologies     string
	installPath      string
	installerUrl     string
	failurePolicy    string
	image            string
	injectOneAgent   bool
	injectDataIngest bool
}

func NewPodInjector(dynakube dynatracev1beta1.DynaKube, technologies string, installPath string, installerUrl string, failurePolicy string, image string) *podInjector {
	return &podInjector{
		dynakube:      dynakube,
		technologies:  technologies,
		installPath:   installPath,
		installerUrl:  installerUrl,
		failurePolicy: failurePolicy,
		image:         image,
	}
}

func (injector *podInjector) Inject(pod corev1.Pod) corev1.Pod {
	injector.pod = &pod

	injector.injectConfigVolume()
	injector.injectOneAgentVolumes()
	injector.injectDataIngestVolumes()

}
