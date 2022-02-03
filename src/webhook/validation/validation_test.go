package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	dynatracev1beta2 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta2"
	dtcsi "github.com/Dynatrace/dynatrace-operator/src/controllers/csi"
	"github.com/Dynatrace/dynatrace-operator/src/scheme/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	testName      = "test-name"
	testNamespace = "test-namespace"
	testApiUrl    = "https://f.q.d.n/api"
)

var defaultDynakubeObjectMeta = metav1.ObjectMeta{
	Name:      testName,
	Namespace: testNamespace,
}

var defaultCSIDaemonSet = appsv1.DaemonSet{
	ObjectMeta: metav1.ObjectMeta{Name: dtcsi.DaemonSetName, Namespace: testNamespace},
}

var dummyLabels = map[string]string{
	"dummy": "label",
}

var dummyNamespace = corev1.Namespace{
	ObjectMeta: metav1.ObjectMeta{
		Name:   "dummy",
		Labels: dummyLabels,
	},
}

var dummyLabels2 = map[string]string{
	"dummy": "label",
}

var dummyNamespace2 = corev1.Namespace{
	ObjectMeta: metav1.ObjectMeta{
		Name:   "dummy2",
		Labels: dummyLabels2,
	},
}

func TestDynakubeValidator_Handle(t *testing.T) {
	t.Run(`valid dynakube specs`, func(t *testing.T) {

		assertAllowedResponseWithWarnings(t, 3, &dynatracev1beta2.DynaKube{
			ObjectMeta: defaultDynakubeObjectMeta,
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: testApiUrl,
				NamespaceSelector: metav1.LabelSelector{
					MatchLabels: dummyLabels,
				},
				OneAgent: dynatracev1beta2.OneAgentSpec{
					CloudNativeFullStack: &dynatracev1beta2.CloudNativeFullStackSpec{
						HostInjectSpec: dynatracev1beta2.HostInjectSpec{
							NodeSelector: map[string]string{
								"node": "1",
							},
						},
					},
				},
				ActiveGates: []dynatracev1beta2.ActiveGateSpec{
					{
						Capabilities: map[dynatracev1beta2.CapabilityDisplayName]dynatracev1beta2.CapabilityProperties{
							dynatracev1beta2.RoutingCapability.DisplayName:       {},
							dynatracev1beta2.KubeMonCapability.DisplayName:       {},
							dynatracev1beta2.MetricsIngestCapability.DisplayName: {},
						},
					},
				},
			}},
			&dynatracev1beta2.DynaKube{
				ObjectMeta: defaultDynakubeObjectMeta,
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					NamespaceSelector: metav1.LabelSelector{
						MatchLabels: dummyLabels2,
					},
					OneAgent: dynatracev1beta2.OneAgentSpec{
						CloudNativeFullStack: &dynatracev1beta2.CloudNativeFullStackSpec{
							HostInjectSpec: dynatracev1beta2.HostInjectSpec{
								NodeSelector: map[string]string{
									"node": "2",
								},
							},
						},
					},
				},
			}, &dummyNamespace, &dummyNamespace2, &defaultCSIDaemonSet)
	})
	t.Run(`conflicting dynakube specs`, func(t *testing.T) {

		assertDeniedResponse(t,
			[]string{
				errorCSIRequired,
				errorNoApiUrl,
				errorConflictingNamespaceSelector,
				fmt.Sprintf(errorInvalidActiveGateCapability, "me dumb"),
				fmt.Sprintf(errorNodeSelectorConflict, "conflict2")},
			&dynatracev1beta2.DynaKube{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName,
					Namespace: testNamespace,
				},
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: "",
					NamespaceSelector: metav1.LabelSelector{
						MatchLabels: dummyLabels,
					},
					OneAgent: dynatracev1beta2.OneAgentSpec{
						CloudNativeFullStack: &dynatracev1beta2.CloudNativeFullStackSpec{},
					},
					ActiveGates: []dynatracev1beta2.ActiveGateSpec{
						{
							Capabilities: map[dynatracev1beta2.CapabilityDisplayName]dynatracev1beta2.CapabilityProperties{
								dynatracev1beta2.KubeMonCapability.DisplayName: {},
								"me dumb": {},
							},
						},
					},
				},
			},
			&dynatracev1beta2.DynaKube{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conflict1",
					Namespace: testNamespace,
				},
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					NamespaceSelector: metav1.LabelSelector{
						MatchLabels: dummyLabels,
					},
					OneAgent: dynatracev1beta2.OneAgentSpec{
						ApplicationMonitoring: &dynatracev1beta2.ApplicationMonitoringSpec{},
					},
				},
			},
			&dynatracev1beta2.DynaKube{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conflict2",
					Namespace: testNamespace,
				},
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					OneAgent: dynatracev1beta2.OneAgentSpec{
						HostMonitoring: &dynatracev1beta2.HostMonitoringSpec{},
					},
				},
			}, &dummyNamespace, &dummyNamespace2)
	})
}

func assertDeniedResponse(t *testing.T, errMessages []string, dynakube *dynatracev1beta2.DynaKube, other ...client.Object) {
	response := handleRequest(t, dynakube, other...)
	assert.False(t, response.Allowed)
	reason := string(response.Result.Reason)
	for _, errMsg := range errMessages {
		assert.Contains(t, reason, errMsg)
	}

}

func assertAllowedResponseWithoutWarnings(t *testing.T, dynakube *dynatracev1beta2.DynaKube, other ...client.Object) {
	response := assertAllowedResponse(t, dynakube, other...)
	assert.Equal(t, len(response.Warnings), 0)
}

func assertAllowedResponseWithWarnings(t *testing.T, warningAmount int, dynakube *dynatracev1beta2.DynaKube, other ...client.Object) {
	response := assertAllowedResponse(t, dynakube, other...)
	assert.Equal(t, len(response.Warnings), warningAmount)
}

func assertAllowedResponse(t *testing.T, dynakube *dynatracev1beta2.DynaKube, other ...client.Object) admission.Response {
	response := handleRequest(t, dynakube, other...)
	assert.True(t, response.Allowed)
	return response
}

func handleRequest(t *testing.T, dynakube *dynatracev1beta2.DynaKube, other ...client.Object) admission.Response {
	clt := fake.NewClient()
	if other != nil {
		clt = fake.NewClient(other...)
	}
	validator := &dynakubeValidator{
		clt:       clt,
		apiReader: clt,
	}

	data, err := json.Marshal(*dynakube)
	require.NoError(t, err)

	return validator.Handle(context.TODO(), admission.Request{
		AdmissionRequest: v1.AdmissionRequest{
			Name:      testName,
			Namespace: testNamespace,
			Object:    runtime.RawExtension{Raw: data},
		},
	})
}

func TestDynakubeValidator_InjectClient(t *testing.T) {
	validator := &dynakubeValidator{}
	clt := fake.NewClient()
	err := validator.InjectClient(clt)

	assert.NoError(t, err)
	assert.NotNil(t, validator.clt)
	assert.Equal(t, clt, validator.clt)
}
