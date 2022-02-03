package validation

import (
	"fmt"
	"testing"

	dynatracev1beta2 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConflictingOneAgentConfiguration(t *testing.T) {
	t.Run(`valid dynakube specs`, func(t *testing.T) {
		assertAllowedResponseWithoutWarnings(t, &dynatracev1beta2.DynaKube{
			ObjectMeta: defaultDynakubeObjectMeta,
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: testApiUrl,
				OneAgent: dynatracev1beta2.OneAgentSpec{
					ClassicFullStack: nil,
					HostMonitoring:   nil,
				},
			},
		})

		assertAllowedResponseWithoutWarnings(t, &dynatracev1beta2.DynaKube{
			ObjectMeta: defaultDynakubeObjectMeta,
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: testApiUrl,
				OneAgent: dynatracev1beta2.OneAgentSpec{
					ClassicFullStack: &dynatracev1beta2.ClassicFullStackSpec{},
					HostMonitoring:   nil,
				},
			},
		})

		assertAllowedResponseWithoutWarnings(t, &dynatracev1beta2.DynaKube{
			ObjectMeta: defaultDynakubeObjectMeta,
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: testApiUrl,
				OneAgent: dynatracev1beta2.OneAgentSpec{
					ClassicFullStack: nil,
					HostMonitoring:   &dynatracev1beta2.HostMonitoringSpec{},
				},
			},
		})
	})
	t.Run(`conflicting dynakube specs`, func(t *testing.T) {
		assertDeniedResponse(t,
			[]string{errorConflictingOneagentMode},
			&dynatracev1beta2.DynaKube{
				ObjectMeta: defaultDynakubeObjectMeta,
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					OneAgent: dynatracev1beta2.OneAgentSpec{
						ClassicFullStack: &dynatracev1beta2.ClassicFullStackSpec{},
						HostMonitoring:   &dynatracev1beta2.HostMonitoringSpec{},
					},
				},
			})

		assertDeniedResponse(t,
			[]string{errorConflictingOneagentMode},
			&dynatracev1beta2.DynaKube{
				ObjectMeta: defaultDynakubeObjectMeta,
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					OneAgent: dynatracev1beta2.OneAgentSpec{
						ApplicationMonitoring: &dynatracev1beta2.ApplicationMonitoringSpec{},
						HostMonitoring:        &dynatracev1beta2.HostMonitoringSpec{},
					},
				},
			})
	})
}

func TestConflictingNodeSelector(t *testing.T) {
	t.Run(`valid dynakube specs`, func(t *testing.T) {
		assertAllowedResponseWithoutWarnings(t,
			&dynatracev1beta2.DynaKube{
				ObjectMeta: defaultDynakubeObjectMeta,
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					OneAgent: dynatracev1beta2.OneAgentSpec{
						HostMonitoring: &dynatracev1beta2.HostMonitoringSpec{
							HostInjectSpec: dynatracev1beta2.HostInjectSpec{
								NodeSelector: map[string]string{
									"node": "1",
								},
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
					OneAgent: dynatracev1beta2.OneAgentSpec{
						HostMonitoring: &dynatracev1beta2.HostMonitoringSpec{
							HostInjectSpec: dynatracev1beta2.HostInjectSpec{
								NodeSelector: map[string]string{
									"node": "2",
								},
							},
						},
					},
				},
			})

		assertAllowedResponseWithWarnings(t, 2,
			&dynatracev1beta2.DynaKube{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conflict2",
					Namespace: testNamespace,
				},
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					OneAgent: dynatracev1beta2.OneAgentSpec{
						CloudNativeFullStack: &dynatracev1beta2.CloudNativeFullStackSpec{
							HostInjectSpec: dynatracev1beta2.HostInjectSpec{
								NodeSelector: map[string]string{
									"node": "1",
								},
							},
						},
					},
				},
			},
			&dynatracev1beta2.DynaKube{
				ObjectMeta: defaultDynakubeObjectMeta,
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					OneAgent: dynatracev1beta2.OneAgentSpec{
						HostMonitoring: &dynatracev1beta2.HostMonitoringSpec{
							HostInjectSpec: dynatracev1beta2.HostInjectSpec{
								NodeSelector: map[string]string{
									"node": "2",
								},
							},
						},
					},
				},
			}, &defaultCSIDaemonSet)
	})
	t.Run(`invalid dynakube specs`, func(t *testing.T) {
		assertDeniedResponse(t,
			[]string{fmt.Sprintf(errorNodeSelectorConflict, "conflicting-dk")},
			&dynatracev1beta2.DynaKube{
				ObjectMeta: defaultDynakubeObjectMeta,
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					OneAgent: dynatracev1beta2.OneAgentSpec{
						CloudNativeFullStack: &dynatracev1beta2.CloudNativeFullStackSpec{
							HostInjectSpec: dynatracev1beta2.HostInjectSpec{
								NodeSelector: map[string]string{
									"node": "1",
								},
							},
						},
					},
				},
			},
			&dynatracev1beta2.DynaKube{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conflicting-dk",
					Namespace: testNamespace,
				},
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					OneAgent: dynatracev1beta2.OneAgentSpec{
						HostMonitoring: &dynatracev1beta2.HostMonitoringSpec{
							HostInjectSpec: dynatracev1beta2.HostInjectSpec{
								NodeSelector: map[string]string{
									"node": "1",
								},
							},
						},
					},
				},
			}, &defaultCSIDaemonSet)
	})
}
