package mutation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	dynatracev1beta1 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta1"
	dtcsi "github.com/Dynatrace/dynatrace-operator/src/controllers/csi"
	oneagent "github.com/Dynatrace/dynatrace-operator/src/controllers/dynakube/oneagent/daemonset"
	"github.com/Dynatrace/dynatrace-operator/src/deploymentmetadata"
	"github.com/Dynatrace/dynatrace-operator/src/dtclient"
	dtingestendpoint "github.com/Dynatrace/dynatrace-operator/src/ingestendpoint"
	"github.com/Dynatrace/dynatrace-operator/src/kubeobjects"
	"github.com/Dynatrace/dynatrace-operator/src/kubesystem"
	dtwebhook "github.com/Dynatrace/dynatrace-operator/src/webhook"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var podLog = log.WithName("pod")

// AddPodMutationWebhookToManager adds the Webhook server to the Manager
func AddPodMutationWebhookToManager(mgr manager.Manager, ns string) error {
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podLog.Info("no Pod name set for webhook container")
	}

	if err := registerInjectEndpoint(mgr, ns, podName); err != nil {
		return err
	}
	registerHealthzEndpoint(mgr)
	return nil
}

func registerInjectEndpoint(mgr manager.Manager, namespace string, podName string) error {
	// Don't use mgr.GetClient() on this function, or other cache-dependent functions from the manager. The cache may
	// not be ready at this point, and queries for Kubernetes objects may fail. mgr.GetAPIReader() doesn't depend on the
	// cache and is safe to use.

	apmExists, err := kubeobjects.CheckIfOneAgentAPMExists(mgr.GetConfig())
	if err != nil {
		return err
	}
	if apmExists {
		podLog.Info(errorOneAgentOperatorExists)
	}

	var pod corev1.Pod
	if err := mgr.GetAPIReader().Get(context.TODO(), client.ObjectKey{
		Name:      podName,
		Namespace: namespace,
	}, &pod); err != nil {
		return err
	}

	var UID types.UID
	if UID, err = kubesystem.GetUID(mgr.GetAPIReader()); err != nil {
		return err
	}

	// the injected podMutator.client doesn't have permissions to Get(sth) from a different namespace
	metaClient, err := client.New(mgr.GetConfig(), client.Options{})
	if err != nil {
		return err
	}

	mgr.GetWebhookServer().Register("/inject", &webhook.Admission{Handler: &podMutator{
		metaClient: metaClient,
		apiReader:  mgr.GetAPIReader(),
		namespace:  namespace,
		image:      pod.Spec.Containers[0].Image,
		apmExists:  apmExists,
		clusterID:  string(UID),
		recorder:   mgr.GetEventRecorderFor("Webhook Server"),
	}})
	return nil
}

func registerHealthzEndpoint(mgr manager.Manager) {
	mgr.GetWebhookServer().Register("/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

// podMutator injects the OneAgent into Pods
type podMutator struct {
	client     client.Client
	metaClient client.Client
	apiReader  client.Reader
	decoder    *admission.Decoder
	image      string
	namespace  string
	apmExists  bool
	clusterID  string
	recorder   record.EventRecorder
}

// Handle adds an annotation to every incoming pods
func (mutator *podMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	injectionReq, admissionResponse := mutator.handleInjectionFeatureDetection(ctx, req)

	if injectionReq == nil {
		return admissionResponse
	}

	injectionInfo := injectionReq.injectionInfo
	dataIngestFields, admissionResponse := mutator.handleInitIngestSecrets(ctx, injectionReq)

	if dataIngestFields == nil {
		return admissionResponse
	}

	dynakube := injectionReq.dynakube
	pod := injectionReq.pod
	podLog.Info("injecting into Pod", "name", pod.Name, "generatedName", pod.GenerateName, "namespace", req.Namespace)

	// ---- TODO: refactor below ----
	response := mutator.handleAlreadyInjectedPod(pod, *dynakube, injectionInfo, dataIngestFields, req)
	if response != nil {
		return *response
	}

	injectionInfo.fillAnnotations(pod)

	workloadName, workloadKind, workloadResponse := mutator.retrieveWorkload(ctx, req, injectionInfo, pod)
	if workloadResponse != nil {
		return *workloadResponse
	}

	technologies := injectionReq.technologies
	installPath := injectionReq.installPath
	installerURL := injectionReq.installerURL
	failurePolicy := injectionReq.failurePolicy
	image := injectionReq.image

	dkVol, mode := ensureDynakubeVolume(*dynakube)
	setupInjectionConfigVolume(pod)
	setupOneAgentVolumes(injectionInfo, pod, dkVol)
	setupDataIngestVolumes(injectionInfo, pod)

	sc := getSecurityContext(pod)
	basePodName := getBasePodName(pod)
	deploymentMetadata := mutator.getDeploymentMetadata(*dynakube)

	// ----- TODO: refactor init container
	installContainer := createInstallInitContainerBase(image, pod, failurePolicy, basePodName, sc, *dynakube)

	decorateInstallContainerWithOneAgent(&installContainer, injectionInfo, technologies, installPath, installerURL, mode)
	decorateInstallContainerWithDataIngest(&installContainer, injectionInfo, workloadKind, workloadName)

	updateContainers(pod, injectionInfo, &installContainer, *dynakube, deploymentMetadata, dataIngestFields)

	addToInitContainers(pod, installContainer)

	mutator.recorder.Eventf(dynakube,
		corev1.EventTypeNormal,
		injectEvent,
		"Injecting the necessary info into pod %s in namespace %s", basePodName, injectionReq.namespace)

	return getResponseForPod(pod, &req)
}

func (mutator *podMutator) handleInjectionFeatureDetection(ctx context.Context, request admission.Request) (*injectionRequest, admission.Response) {
	if mutator.apmExists {
		return nil, admission.Patched(errorOneAgentOperatorExists)
	}

	injectionReq, err := createInjectionRequest(ctx, request, mutator)
	if err != nil {
		return nil, admission.Errored(injectionReq.errorCode, err)
	}

	injectionInfo := injectionReq.injectionInfo
	if !injectionInfo.hasAnyEnabled() {
		return nil, admission.Patched(errorNoFeaturesEnabled)
	}

	dynakube := injectionReq.dynakube
	if dynakube.FeatureDisableMetadataEnrichment() {
		injectionInfo.features[DataIngest] = false
	}

	if !dynakube.NeedAppInjection() {
		return nil, admission.Patched(errorAppInjectionDisabled)
	}

	return injectionReq, admission.Response{}
}

func (mutator *podMutator) handleInitIngestSecrets(ctx context.Context, injectionReq *injectionRequest) (map[string]string, admission.Response) {
	dynakube := injectionReq.dynakube
	injectionInfo := injectionReq.injectionInfo
	dataIngestFields := map[string]string{}
	secrets := newIngestInitSecrets(ctx, mutator.client, mutator.apiReader, dynakube, mutator.namespace)
	err := secrets.createInitSecretIfNotExists()

	if err != nil {
		return nil, admission.Errored(http.StatusBadRequest, err)
	}

	if injectionInfo.enabled(DataIngest) {
		endpointSecretGenerator := dtingestendpoint.NewEndpointSecretGenerator(mutator.client, mutator.apiReader, mutator.namespace)
		err = secrets.createDataIngestSecretIfNotExists(endpointSecretGenerator)
		if err != nil {
			return nil, admission.Errored(http.StatusBadRequest, err)
		}

		dataIngestFields, err = endpointSecretGenerator.PrepareFields(ctx, dynakube)
		if err != nil {
			return nil, admission.Errored(http.StatusBadRequest, err)
		}
	}

	return dataIngestFields, admission.Response{}
}

func findRootOwnerOfPod(ctx context.Context, clt client.Client, pod *corev1.Pod, namespace string) (string, string, error) {
	obj := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pod.APIVersion,
			Kind:       pod.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: pod.ObjectMeta.Name,
			// pod.ObjectMeta.Namespace is empty yet
			Namespace:       namespace,
			OwnerReferences: pod.ObjectMeta.OwnerReferences,
		},
	}
	return findRootOwner(ctx, clt, obj)
}

func findRootOwner(ctx context.Context, clt client.Client, o *metav1.PartialObjectMetadata) (string, string, error) {
	if len(o.ObjectMeta.OwnerReferences) == 0 {
		kind := o.Kind
		if kind == "Pod" {
			kind = ""
		}
		return o.ObjectMeta.Name, kind, nil
	}

	om := o.ObjectMeta
	for _, owner := range om.OwnerReferences {
		if owner.Controller != nil && *owner.Controller && isWellKnownWorkload(owner) {
			obj := &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					APIVersion: owner.APIVersion,
					Kind:       owner.Kind,
				},
			}
			if err := clt.Get(ctx, client.ObjectKey{Name: owner.Name, Namespace: om.Namespace}, obj); err != nil {
				podLog.Error(err, "failed to query the object", "apiVersion", owner.APIVersion, "kind", owner.Kind, "name", owner.Name, "namespace", om.Namespace)
				return o.ObjectMeta.Name, o.Kind, err
			}

			return findRootOwner(ctx, clt, obj)
		}
	}
	return o.ObjectMeta.Name, o.Kind, nil
}

func isWellKnownWorkload(ownerRef metav1.OwnerReference) bool {
	knownWorkloads := []metav1.TypeMeta{
		{Kind: "ReplicaSet", APIVersion: "apps/v1"},
		{Kind: "Deployment", APIVersion: "apps/v1"},
		{Kind: "ReplicationController", APIVersion: "v1"},
		{Kind: "StatefulSet", APIVersion: "apps/v1"},
		{Kind: "DaemonSet", APIVersion: "apps/v1"},
		{Kind: "Job", APIVersion: "batch/v1"},
		{Kind: "CronJob", APIVersion: "batch/v1"},
		{Kind: "DeploymentConfig", APIVersion: "apps.openshift.io/v1"},
	}

	for _, knownController := range knownWorkloads {
		if ownerRef.Kind == knownController.Kind &&
			ownerRef.APIVersion == knownController.APIVersion {
			return true
		}
	}
	return false
}

func (mutator *podMutator) handleAlreadyInjectedPod(pod *corev1.Pod, dk dynatracev1beta1.DynaKube, injectionInfo *InjectionInfo, dataIngestFields map[string]string, req admission.Request) *admission.Response {
	// are there any injections already?
	if len(pod.Annotations[dtwebhook.AnnotationDynatraceInjected]) > 0 {
		if dk.FeatureEnableWebhookReinvocationPolicy() {
			rsp := mutator.applyReinvocationPolicy(pod, dk, injectionInfo, dataIngestFields, req)
			return &rsp
		}
		rsp := admission.Patched("")
		return &rsp
	}
	return nil
}

func addToInitContainers(pod *corev1.Pod, installContainer corev1.Container) {
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, installContainer)
}

func updateContainers(pod *corev1.Pod, injectionInfo *InjectionInfo, ic *corev1.Container, dk dynatracev1beta1.DynaKube, deploymentMetadata *deploymentmetadata.DeploymentMetadata, dataIngestFields map[string]string) {
	for i := range pod.Spec.Containers {
		c := &pod.Spec.Containers[i]

		if injectionInfo.enabled(OneAgent) {
			updateInstallContainerOneAgent(ic, i+1, c.Name, c.Image)
			updateContainerOneAgent(c, &dk, pod, deploymentMetadata)
		}
		if injectionInfo.enabled(DataIngest) {
			updateContainerDataIngest(c, pod, deploymentMetadata, dataIngestFields)
		}
	}
}

func decorateInstallContainerWithDataIngest(ic *corev1.Container, injectionInfo *InjectionInfo, workloadKind string, workloadName string) {
	if injectionInfo.enabled(DataIngest) {
		ic.Env = append(ic.Env,
			corev1.EnvVar{Name: workloadKindEnvVarName, Value: workloadKind},
			corev1.EnvVar{Name: workloadNameEnvVarName, Value: workloadName},
			corev1.EnvVar{Name: dataIngestInjectedEnvVarName, Value: "true"},
		)

		ic.VolumeMounts = append(ic.VolumeMounts, corev1.VolumeMount{
			Name:      dataIngestVolumeName,
			MountPath: dataIngestMountPath})
	} else {
		ic.Env = append(ic.Env,
			corev1.EnvVar{Name: dataIngestInjectedEnvVarName, Value: "false"},
		)
	}
}

func decorateInstallContainerWithOneAgent(ic *corev1.Container, injectionInfo *InjectionInfo, technologies string, installPath string, installerURL string, mode string) {
	if injectionInfo.enabled(OneAgent) {
		ic.Env = append(ic.Env,
			corev1.EnvVar{Name: "FLAVOR", Value: dtclient.FlavorMultidistro},
			corev1.EnvVar{Name: "TECHNOLOGIES", Value: technologies},
			corev1.EnvVar{Name: "INSTALLPATH", Value: installPath},
			corev1.EnvVar{Name: "INSTALLER_URL", Value: installerURL},
			corev1.EnvVar{Name: "MODE", Value: mode},
			corev1.EnvVar{Name: oneAgentInjectedEnvVarName, Value: "true"},
		)

		ic.VolumeMounts = append(ic.VolumeMounts,
			corev1.VolumeMount{Name: oneAgentBinVolumeName, MountPath: "/mnt/bin"},
			corev1.VolumeMount{Name: oneAgentShareVolumeName, MountPath: "/mnt/share"},
		)
	} else {
		ic.Env = append(ic.Env,
			corev1.EnvVar{Name: oneAgentInjectedEnvVarName, Value: "false"},
		)
	}
}

func createInstallInitContainerBase(image string, pod *corev1.Pod, failurePolicy string, basePodName string, sc *corev1.SecurityContext, dk dynatracev1beta1.DynaKube) corev1.Container {
	ic := corev1.Container{
		Name:            dtwebhook.InstallContainerName,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/usr/bin/env"},
		Args:            []string{"bash", "/mnt/config/init.sh"},
		Env: []corev1.EnvVar{
			{Name: "CONTAINERS_COUNT", Value: strconv.Itoa(len(pod.Spec.Containers))},
			{Name: "FAILURE_POLICY", Value: failurePolicy},
			{Name: "K8S_PODNAME", ValueFrom: fieldEnvVar("metadata.name")},
			{Name: "K8S_PODUID", ValueFrom: fieldEnvVar("metadata.uid")},
			{Name: "K8S_BASEPODNAME", Value: basePodName},
			{Name: "K8S_NAMESPACE", ValueFrom: fieldEnvVar("metadata.namespace")},
			{Name: "K8S_NODE_NAME", ValueFrom: fieldEnvVar("spec.nodeName")},
		},
		SecurityContext: sc,
		VolumeMounts: []corev1.VolumeMount{
			{Name: injectionConfigVolumeName, MountPath: "/mnt/config"},
		},
		Resources: *dk.InitResources(),
	}
	return ic
}

func (mutator *podMutator) getDeploymentMetadata(dk dynatracev1beta1.DynaKube) *deploymentmetadata.DeploymentMetadata {
	var deploymentMetadata *deploymentmetadata.DeploymentMetadata
	if dk.CloudNativeFullstackMode() {
		deploymentMetadata = deploymentmetadata.NewDeploymentMetadata(mutator.clusterID, deploymentmetadata.DeploymentTypeCloudNative)
	} else {
		deploymentMetadata = deploymentmetadata.NewDeploymentMetadata(mutator.clusterID, deploymentmetadata.DeploymentTypeApplicationMonitoring)
	}
	return deploymentMetadata
}

func getBasePodName(pod *corev1.Pod) string {
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

func getSecurityContext(pod *corev1.Pod) *corev1.SecurityContext {
	var sc *corev1.SecurityContext
	if pod.Spec.Containers[0].SecurityContext != nil {
		sc = pod.Spec.Containers[0].SecurityContext.DeepCopy()
	}
	return sc
}

func (mutator *podMutator) retrieveWorkload(ctx context.Context, req admission.Request, injectionInfo *InjectionInfo, pod *corev1.Pod) (string, string, *admission.Response) {
	var rsp admission.Response
	var workloadName, workloadKind string
	if injectionInfo.enabled(DataIngest) {
		var err error
		workloadName, workloadKind, err = findRootOwnerOfPod(ctx, mutator.metaClient, pod, req.Namespace)
		if err != nil {
			rsp = admission.Errored(http.StatusInternalServerError, err)
			return "", "", &rsp
		}
	}
	return workloadName, workloadKind, nil
}

func (mutator *podMutator) applyReinvocationPolicy(pod *corev1.Pod, dk dynatracev1beta1.DynaKube, injectionInfo *InjectionInfo, dataIngestFields map[string]string, req admission.Request) admission.Response {
	var needsUpdate = false
	var installContainer *corev1.Container
	for i := range pod.Spec.Containers {
		c := &pod.Spec.Containers[i]

		oaInjected := false
		if injectionInfo.enabled(OneAgent) {
			for _, e := range c.Env {
				if e.Name == "LD_PRELOAD" {
					oaInjected = true
					break
				}
			}
		}
		diInjected := false
		if injectionInfo.enabled(DataIngest) {
			for _, vm := range c.VolumeMounts {
				if vm.Name == dataIngestEndpointVolumeName {
					diInjected = true
					break
				}
			}
		}

		oaInjectionMissing := injectionInfo.enabled(OneAgent) && !oaInjected
		diInjectionMissing := injectionInfo.enabled(DataIngest) && !diInjected

		if oaInjectionMissing {
			// container does not have LD_PRELOAD set
			podLog.Info("instrumenting missing container", "injectable", "oneagent", "name", c.Name)

			deploymentMetadata := deploymentmetadata.NewDeploymentMetadata(mutator.clusterID, deploymentmetadata.DeploymentTypeApplicationMonitoring)

			updateContainerOneAgent(c, &dk, pod, deploymentMetadata)

			if installContainer == nil {
				for j := range pod.Spec.InitContainers {
					ic := &pod.Spec.InitContainers[j]

					if ic.Name == dtwebhook.InstallContainerName {
						installContainer = ic
						break
					}
				}
			}
			updateInstallContainerOneAgent(installContainer, i+1, c.Name, c.Image)

			needsUpdate = true
		}

		if diInjectionMissing {
			podLog.Info("instrumenting missing container", "injectable", "data-ingest", "name", c.Name)

			deploymentMetadata := deploymentmetadata.NewDeploymentMetadata(mutator.clusterID, deploymentmetadata.DeploymentTypeApplicationMonitoring)
			updateContainerDataIngest(c, pod, deploymentMetadata, dataIngestFields)

			needsUpdate = true
		}
	}

	if needsUpdate {
		podLog.Info("updating pod with missing containers")
		mutator.recorder.Eventf(&dk,
			corev1.EventTypeNormal,
			updatePodEvent,
			"Updating pod %s in namespace %s with missing containers", pod.GenerateName, pod.Namespace)
		return getResponseForPod(pod, &req)
	}
	return admission.Patched("")
}

// InjectClient injects the client
func (mutator *podMutator) InjectClient(c client.Client) error {
	mutator.client = c
	return nil
}

// InjectDecoder injects the decoder
func (mutator *podMutator) InjectDecoder(d *admission.Decoder) error {
	mutator.decoder = d
	return nil
}

// updateInstallContainerOA adds Container to list of Containers of Install Container
func updateInstallContainerOneAgent(ic *corev1.Container, number int, name string, image string) {
	podLog.Info("updating install container with new container", "containerName", name, "containerImage", image)
	ic.Env = append(ic.Env,
		corev1.EnvVar{Name: fmt.Sprintf("CONTAINER_%d_NAME", number), Value: name},
		corev1.EnvVar{Name: fmt.Sprintf("CONTAINER_%d_IMAGE", number), Value: image})
}

// updateContainerOA sets missing preload Variables
func updateContainerOneAgent(c *corev1.Container, dk *dynatracev1beta1.DynaKube, pod *corev1.Pod, deploymentMetadata *deploymentmetadata.DeploymentMetadata) {

	podLog.Info("updating container with missing preload variables", "containerName", c.Name)
	installPath := kubeobjects.GetField(pod.Annotations, dtwebhook.AnnotationInstallPath, dtwebhook.DefaultInstallPath)

	addMetadataIfMissing(c, deploymentMetadata)

	c.VolumeMounts = append(c.VolumeMounts,
		corev1.VolumeMount{
			Name:      oneAgentShareVolumeName,
			MountPath: "/etc/ld.so.preload",
			SubPath:   "ld.so.preload",
		},
		corev1.VolumeMount{
			Name:      oneAgentBinVolumeName,
			MountPath: installPath,
		},
		corev1.VolumeMount{
			Name:      oneAgentShareVolumeName,
			MountPath: "/var/lib/dynatrace/oneagent/agent/config/container.conf",
			SubPath:   fmt.Sprintf("container_%s.conf", c.Name),
		})
	if dk.HasActiveGateTLS() {
		c.VolumeMounts = append(c.VolumeMounts,
			corev1.VolumeMount{
				Name:      oneAgentShareVolumeName,
				MountPath: filepath.Join(oneagent.OneAgentCustomKeysPath, "custom.pem"),
				SubPath:   "custom.pem",
			})
	}

	c.Env = append(c.Env,
		corev1.EnvVar{
			Name:  "LD_PRELOAD",
			Value: installPath + "/agent/lib64/liboneagentproc.so",
		})

	if dk.Spec.Proxy != nil && (dk.Spec.Proxy.Value != "" || dk.Spec.Proxy.ValueFrom != "") {
		c.Env = append(c.Env,
			corev1.EnvVar{
				Name: "DT_PROXY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: dtwebhook.SecretConfigName,
						},
						Key: "proxy",
					},
				},
			})
	}

	if dk.Spec.NetworkZone != "" {
		c.Env = append(c.Env, corev1.EnvVar{Name: "DT_NETWORK_ZONE", Value: dk.Spec.NetworkZone})
	}

}

func addMetadataIfMissing(c *corev1.Container, deploymentMetadata *deploymentmetadata.DeploymentMetadata) {
	for _, v := range c.Env {
		if v.Name == dynatraceMetadataEnvVarName {
			return
		}
	}

	c.Env = append(c.Env,
		corev1.EnvVar{
			Name:  dynatraceMetadataEnvVarName,
			Value: deploymentMetadata.AsString(),
		})
}

func updateContainerDataIngest(c *corev1.Container, pod *corev1.Pod, deploymentMetadata *deploymentmetadata.DeploymentMetadata, dataIngestFields map[string]string) {
	podLog.Info("updating container with missing data ingest enrichment", "containerName", c.Name)

	addMetadataIfMissing(c, deploymentMetadata)

	c.VolumeMounts = append(c.VolumeMounts,
		corev1.VolumeMount{
			Name:      dataIngestVolumeName,
			MountPath: "/var/lib/dynatrace/enrichment",
		},
		corev1.VolumeMount{
			Name:      dataIngestEndpointVolumeName,
			MountPath: "/var/lib/dynatrace/enrichment/endpoint",
		},
	)

	c.Env = append(c.Env,
		corev1.EnvVar{
			Name:  dtingestendpoint.UrlSecretField,
			Value: dataIngestFields[dtingestendpoint.UrlSecretField],
		},
		corev1.EnvVar{
			Name:  dtingestendpoint.TokenSecretField,
			Value: dataIngestFields[dtingestendpoint.TokenSecretField],
		},
	)
}

// getResponseForPod tries to format pod as json
func getResponseForPod(pod *corev1.Pod, req *admission.Request) admission.Response {
	marshaledPod, err := json.MarshalIndent(pod, "", "  ")
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func fieldEnvVar(key string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: key}}
}
