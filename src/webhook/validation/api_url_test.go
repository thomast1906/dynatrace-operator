package validation

import (
	"strings"
	"testing"

	dynatracev1beta2 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestHasApiUrl(t *testing.T) {
	instance := &dynatracev1beta2.DynaKube{}
	assert.Equal(t, errorNoApiUrl, noApiUrl(nil, instance))

	instance.Spec.APIURL = testApiUrl
	assert.Empty(t, noApiUrl(nil, instance))

	t.Run(`happy path`, func(t *testing.T) {
		assertAllowedResponse(t, &dynatracev1beta2.DynaKube{
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: "https://tenantid.doma.in/api",
			},
		})
	})
	t.Run(`missing API URL`, func(t *testing.T) {
		assertDeniedResponse(t, []string{errorNoApiUrl}, &dynatracev1beta2.DynaKube{
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: "",
			},
		})
	})
	t.Run(`invalid API URL`, func(t *testing.T) {
		assertDeniedResponse(t, []string{errorNoApiUrl}, &dynatracev1beta2.DynaKube{
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: exampleApiUrl,
			},
		})
	})
	t.Run(`invalid API URL (without /api suffix)`, func(t *testing.T) {
		assertDeniedResponse(t, []string{errorInvalidApiUrl}, &dynatracev1beta2.DynaKube{
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: strings.TrimSuffix(exampleApiUrl, "/api"),
			},
		})
	})
	t.Run(`invalid API URL (not a Dynatrace environment)`, func(t *testing.T) {
		assertDeniedResponse(t, []string{errorInvalidApiUrl}, &dynatracev1beta2.DynaKube{
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: "https://www.google.com",
			},
		})
	})
	t.Run(`invalid API URL (empty tenant ID)`, func(t *testing.T) {
		assertDeniedResponse(t, []string{errorInvalidApiUrl}, &dynatracev1beta2.DynaKube{
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: "/api",
			},
		})
	})
	t.Run(`invalid API URL (missing domain)`, func(t *testing.T) {
		assertDeniedResponse(t, []string{errorInvalidApiUrl}, &dynatracev1beta2.DynaKube{
			Spec: dynatracev1beta2.DynaKubeSpec{
				APIURL: "https://...tenantid/api",
			},
		})
	})
}
