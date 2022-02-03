package nodes

import (
	"context"
	"os"

	dynatracev1beta2 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *NodesController) determineDynakubeForNode(nodeName string) (*dynatracev1beta2.DynaKube, error) {
	dkList, err := r.getOneAgentList()
	if err != nil {
		return nil, err
	}

	return r.filterOneAgentFromList(dkList, nodeName), nil
}

func (r *NodesController) getOneAgentList() (*dynatracev1beta2.DynaKubeList, error) {
	watchNamespace := os.Getenv("POD_NAMESPACE")

	var dkList dynatracev1beta2.DynaKubeList
	err := r.client.List(context.TODO(), &dkList, client.InNamespace(watchNamespace))
	if err != nil {
		return nil, err
	}

	return &dkList, nil
}

func (r *NodesController) filterOneAgentFromList(dkList *dynatracev1beta2.DynaKubeList,
	nodeName string) *dynatracev1beta2.DynaKube {

	for _, dynakube := range dkList.Items {
		items := dynakube.Status.OneAgent.Instances
		if _, ok := items[nodeName]; ok {
			return &dynakube
		}
	}

	return nil
}
