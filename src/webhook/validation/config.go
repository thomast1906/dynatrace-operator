package validation

import (
	dynatracev1beta2 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta2"
	"github.com/Dynatrace/dynatrace-operator/src/logger"
)

var log = logger.NewDTLogger().WithName("validation-webhook")

type validator func(dv *dynakubeValidator, dynakube *dynatracev1beta2.DynaKube) string

var validators = []validator{
	noApiUrl,
	isInvalidApiUrl,
	missingCSIDaemonSet,
	invalidActiveGateCapabilities,
	duplicateActiveGateCapabilities,
	conflictingOneAgentConfiguration,
	conflictingNodeSelector,
	conflictingNamespaceSelector,
}

var warnings = []validator{
	oneAgentModePreviewWarning,
	metricIngestPreviewWarning,
}
