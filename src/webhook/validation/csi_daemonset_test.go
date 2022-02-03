package validation

import (
	"testing"

	dynatracev1beta2 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta2"
)

func TestMissingCSIDaemonSet(t *testing.T) {
	t.Run(`valid dynakube specs`, func(t *testing.T) {
		assertAllowedResponseWithWarnings(t, 2, &dynatracev1beta2.DynaKube{
			ObjectMeta: defaultDynakubeObjectMeta,
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: testApiUrl,
				OneAgent: dynatracev1beta2.OneAgentSpec{
					CloudNativeFullStack: &dynatracev1beta2.CloudNativeFullStackSpec{},
				},
			},
		}, &defaultCSIDaemonSet)
	})

	t.Run(`invalid dynakube specs`, func(t *testing.T) {
		assertDeniedResponse(t,
			[]string{errorCSIRequired},
			&dynatracev1beta2.DynaKube{
				ObjectMeta: defaultDynakubeObjectMeta,
				Spec: dynatracev1beta2.DynaKubeSpec{
					APIURL: testApiUrl,
					OneAgent: dynatracev1beta2.OneAgentSpec{
						CloudNativeFullStack: &dynatracev1beta2.CloudNativeFullStackSpec{},
					},
				},
			})
	})
}
