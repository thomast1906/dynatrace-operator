package validation

import (
	"fmt"

	dynatracev1beta2 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta2"
)

const (
	errorInvalidActiveGateCapability = `The DynaKube's specification tries to use an invalid capability in ActiveGate section, invalid capability=%s.
Make sure you correctly specify the ActiveGate capabilities in your custom resource.
`

	errorDuplicateActiveGateCapability = `The DynaKube's specification tries to specify duplicate capabilities in the ActiveGate section, duplicate capability=%s.
Make sure you don't duplicate an Activegate capability in your custom resource.
`
)

func duplicateActiveGateCapabilities(dv *dynakubeValidator, dynakube *dynatracev1beta2.DynaKube) string {
	if dynakube.ActiveGateMode() {
		for _, activeGate := range dynakube.Spec.ActiveGates {
			capabilities := activeGate.Capabilities
			duplicateChecker := map[dynatracev1beta2.CapabilityDisplayName]bool{}
			for capabilityName := range capabilities {
				if duplicateChecker[capabilityName] {
					log.Info("requested dynakube has duplicates in the active gate capabilities section", "name", dynakube.Name, "namespace", dynakube.Namespace, "activegate", activeGate.Name)
					return fmt.Sprintf(errorDuplicateActiveGateCapability, capabilityName)
				}
				duplicateChecker[capabilityName] = true
			}
		}
	}
	return ""
}

func invalidActiveGateCapabilities(dv *dynakubeValidator, dynakube *dynatracev1beta2.DynaKube) string {
	if dynakube.ActiveGateMode() {
		for _, activeGate := range dynakube.Spec.ActiveGates {
			capabilities := activeGate.Capabilities
			for capabilityName := range capabilities {
				if _, ok := dynatracev1beta2.ActiveGateDisplayNames[capabilityName]; !ok {
					log.Info("requested dynakube has invalid active gate capability", "name", dynakube.Name, "namespace", dynakube.Namespace, "activegate", activeGate.Name)
					return fmt.Sprintf(errorInvalidActiveGateCapability, capabilityName)
				}
			}
		}
	}
	return ""
}
