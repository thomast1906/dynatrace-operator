package mutation

import (
	"context"
	"github.com/Dynatrace/dynatrace-operator/src/api/v1beta1"
	"github.com/Dynatrace/dynatrace-operator/src/mapper"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type injectionRequest struct {
	pod           *corev1.Pod
	dynakube      *v1beta1.DynaKube
	namespace     *corev1.Namespace
	request       admission.Request
	ctx           context.Context
	injectionInfo *InjectionInfo
	mutator       *podMutator
	errorCode     int32
	*basicData
}

func createInjectionRequest(ctx context.Context, req admission.Request, mutator *podMutator) (*injectionRequest, error) {
	result := &injectionRequest{
		ctx:     ctx,
		request: req,
		mutator: mutator,
	}
	pod, err := result.decodePod()
	result.errorCode = http.StatusInternalServerError

	if err != nil {
		log.Error(err, errorDecodingPod)
		return result, errors.WithStack(err)
	}

	result.pod = pod
	result.injectionInfo = NewInjectionInfoForPod(result.pod)
	result.basicData = createBasicDataFromPod(result.pod, mutator.image)
	namespace, err := result.findNamespace()

	if err != nil {
		log.Error(err, errorFailedToQueryNamespace)
		return result, errors.WithStack(err)
	}

	result.namespace = namespace
	dynakube, err := result.findDynakube()

	if err != nil {
		return result, errors.WithStack(err)
	}

	result.dynakube = dynakube
	result.errorCode = http.StatusOK
	return result, nil
}

func (injectionRequest *injectionRequest) decodePod() (*corev1.Pod, error) {
	mutator := injectionRequest.mutator
	request := injectionRequest.request
	pod := &corev1.Pod{}
	err := mutator.decoder.Decode(request, pod)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return pod, nil
}

func (injectionRequest *injectionRequest) findNamespace() (*corev1.Namespace, error) {
	ctx := injectionRequest.ctx
	namespaceName := injectionRequest.request.Namespace
	mutator := injectionRequest.mutator

	var namespace corev1.Namespace
	err := mutator.client.Get(ctx, client.ObjectKey{Name: namespaceName}, &namespace)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &namespace, nil
}

func (injectionRequest *injectionRequest) findDynakube() (*v1beta1.DynaKube, error) {
	mutator := injectionRequest.mutator
	ctx := injectionRequest.ctx
	namespace := injectionRequest.namespace
	dynakubeName, hasDynakubeLabel := namespace.Labels[mapper.InstanceLabel]

	if !hasDynakubeLabel {
		injectionRequest.errorCode = http.StatusBadRequest
		return nil, errors.New(errorDynakubeLabelNotSet(namespace.Name))
	}

	var dynakube v1beta1.DynaKube
	err := mutator.client.Get(ctx, client.ObjectKey{Name: dynakubeName, Namespace: namespace.Name}, &dynakube)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			injectionRequest.errorCode = http.StatusBadRequest
			log.Error(err, errorDynakubeAssignedButDoesNotExist(dynakubeName, namespace.Name))
		} else {
			log.Error(err, errorFailedToQueryDynakube)
		}
		return nil, errors.WithStack(err)
	}

	return &dynakube, nil
}
