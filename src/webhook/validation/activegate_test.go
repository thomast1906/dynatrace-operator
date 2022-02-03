package validation

import (
	"fmt"
	"testing"

	dynatracev1beta2 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta2"
)

func TestConflictingActiveGateConfiguration(t *testing.T) {
	t.Run(`valid dynakube specs`, func(t *testing.T) {

		assertAllowedResponseWithoutWarnings(t, &dynatracev1beta2.DynaKube{
			ObjectMeta: defaultDynakubeObjectMeta,
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: testApiUrl,
				ActiveGates: []dynatracev1beta2.ActiveGateSpec{
					{
						Capabilities: map[dynatracev1beta2.CapabilityDisplayName]dynatracev1beta2.CapabilityProperties{
							dynatracev1beta2.RoutingCapability.DisplayName: {},
							dynatracev1beta2.KubeMonCapability.DisplayName: {},
						},
					},
				},
			},
		})

		assertAllowedResponseWithWarnings(t, 2, &dynatracev1beta2.DynaKube{
			ObjectMeta: defaultDynakubeObjectMeta,
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: testApiUrl,
				ActiveGates: []dynatracev1beta2.ActiveGateSpec{
					{
						Capabilities: map[dynatracev1beta2.CapabilityDisplayName]dynatracev1beta2.CapabilityProperties{
							dynatracev1beta2.MetricsIngestCapability.DisplayName: {},
						},
					},
				},
			},
		})
	})
}

func TestDuplicateActiveGateCapabilities(t *testing.T) {
	/*
		There is no way to insert 2 routing capabilities into Capabilities MAP

		t.Run(`conflicting dynakube specs`, func(t *testing.T) {
			assertDeniedResponse(t,
				[]string{fmt.Sprintf(errorDuplicateActiveGateCapability, dynatracev1beta2.RoutingCapability.DisplayName)},
				&dynatracev1beta2.DynaKube{
					ObjectMeta: defaultDynakubeObjectMeta,
					Spec: dynatracev1beta2.DynaKubeSpec{
						APIURL: testApiUrl,
						ActiveGates: []dynatracev1beta2.ActiveGateSpec{
							{
								Capabilities: map[dynatracev1beta2.CapabilityDisplayName]dynatracev1beta2.CapabilityProperties{
									dynatracev1beta2.RoutingCapability.DisplayName: {},
									dynatracev1beta2.RoutingCapability.DisplayName: {},
								},
							},
						},
					},
				})
		})
	*/
}

func TestInvalidActiveGateCapabilities(t *testing.T) {

	t.Run(`conflicting dynakube specs`, func(t *testing.T) {
		assertDeniedResponse(t,
			[]string{fmt.Sprintf(errorInvalidActiveGateCapability, "invalid-capability")},
			&dynatracev1beta2.DynaKube{
				ObjectMeta: defaultDynakubeObjectMeta,
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					ActiveGates: []dynatracev1beta2.ActiveGateSpec{
						{
							Capabilities: map[dynatracev1beta2.CapabilityDisplayName]dynatracev1beta2.CapabilityProperties{
								"invalid-capability": {},
							},
						},
					},
				},
			})
	})
}
