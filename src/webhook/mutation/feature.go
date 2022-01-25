package mutation

import (
	dtwebhook "github.com/Dynatrace/dynatrace-operator/src/webhook"
	"strconv"
)

type FeatureType int

const (
	OneAgent FeatureType = iota
	DataIngest
)

type Feature struct {
	featureType FeatureType
	enabled     bool
}

func NewFeature(featureType FeatureType, enabled bool) Feature {
	return Feature{featureType: featureType, enabled: enabled}
}

func (feature Feature) annotationValue() string {
	return strconv.FormatBool(feature.enabled)
}

func (feature FeatureType) name() string {
	annotationName := "unknown"
	switch feature {
	case OneAgent:
		annotationName = dtwebhook.OneAgentPrefix
	case DataIngest:
		annotationName = dtwebhook.DataIngestPrefix
	}
	return annotationName
}

// for testing only
func (feature FeatureType) namePrefixed() string {
	annotationName := "unknown"
	switch feature {
	case OneAgent:
		annotationName = dtwebhook.AnnotationOneAgentInject
	case DataIngest:
		annotationName = dtwebhook.AnnotationDataIngestInject
	}
	return annotationName
}
