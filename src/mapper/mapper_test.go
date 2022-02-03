package mapper

import (
	dynatracev1beta2 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createTestDynakubeWithAppInject(name string, labels map[string]string, labelExpression []metav1.LabelSelectorRequirement) *dynatracev1beta2.DynaKube {
	dk := &dynatracev1beta2.DynaKube{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "dynatrace"},
		Spec: dynatracev1beta2.DynaKubeSpec{
			OneAgent: dynatracev1beta2.OneAgentSpec{
				ApplicationMonitoring: &dynatracev1beta2.ApplicationMonitoringSpec{},
			},
		},
	}
	if labels != nil {
		dk.Spec.NamespaceSelector = metav1.LabelSelector{MatchLabels: labels}
	}
	if labelExpression != nil {
		dk.Spec.NamespaceSelector = metav1.LabelSelector{MatchExpressions: labelExpression}
	}
	return dk
}

func createTestDynakubeWithMultipleFeatures(name string, labels map[string]string, labelExpression []metav1.LabelSelectorRequirement) *dynatracev1beta2.DynaKube {
	dk := createTestDynakubeWithAppInject(name, labels, labelExpression)
	dk.Spec.ActiveGates = []dynatracev1beta2.ActiveGateSpec{
		{
			Capabilities: map[dynatracev1beta2.CapabilityDisplayName]dynatracev1beta2.CapabilityProperties{
				dynatracev1beta2.RoutingCapability.DisplayName: {},
			},
		},
	}
	return dk
}

func createNamespace(name string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}
