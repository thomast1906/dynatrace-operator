package mutation

import (
	"context"
	"github.com/Dynatrace/dynatrace-operator/src/api/v1beta1"
	dtingestendpoint "github.com/Dynatrace/dynatrace-operator/src/ingestendpoint"
	"github.com/Dynatrace/dynatrace-operator/src/initgeneration"
	dtwebhook "github.com/Dynatrace/dynatrace-operator/src/webhook"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ingestInitSecrets struct {
	ctx       context.Context
	clt       client.Client
	apiReader client.Reader
	dynakube  *v1beta1.DynaKube
	namespace string
}

func newIngestInitSecrets(ctx context.Context, client client.Client, apiReader client.Reader, dynakube *v1beta1.DynaKube, namespace string) *ingestInitSecrets {
	return &ingestInitSecrets{
		ctx:       ctx,
		clt:       client,
		apiReader: apiReader,
		dynakube:  dynakube,
		namespace: namespace,
	}
}

func (secrets *ingestInitSecrets) createInitSecretIfNotExists() error {
	var initSecret corev1.Secret
	ctx := secrets.ctx
	apiReader := secrets.apiReader
	namespace := secrets.namespace
	err := apiReader.Get(ctx, client.ObjectKey{Name: dtwebhook.SecretConfigName, Namespace: namespace}, &initSecret)

	if k8serrors.IsNotFound(err) {
		err = secrets.createInitSecret()

		if err != nil {
			podLog.Error(err, errorCreatingInitSecret)
			return errors.WithStack(err)
		}
	} else if err != nil {
		podLog.Error(err, errorFailedToQueryInitSecret)
		return errors.WithStack(err)
	}

	return nil
}

func (secrets *ingestInitSecrets) createInitSecret() error {
	ctx := secrets.ctx
	clt := secrets.clt
	apiReader := secrets.apiReader
	dynakube := secrets.dynakube
	namespace := secrets.namespace
	_, err := initgeneration.NewInitGenerator(clt, apiReader, namespace).
		GenerateForNamespace(ctx, *dynakube, dynakube.GetName())

	return errors.WithStack(err)
}

func (secrets *ingestInitSecrets) createDataIngestSecretIfNotExists(endpointSecretGenerator *dtingestendpoint.EndpointSecretGenerator) error {
	var endpointSecret corev1.Secret
	ctx := secrets.ctx
	apiReader := secrets.apiReader
	namespace := secrets.namespace
	err := apiReader.Get(ctx, client.ObjectKey{Name: dtingestendpoint.SecretEndpointName, Namespace: namespace}, &endpointSecret)

	if k8serrors.IsNotFound(err) {
		err = secrets.createDataIngestSecret(endpointSecretGenerator)

		if err != nil {
			podLog.Error(err, errorCreatingQueryDataIngestEndpointSecret)
			return errors.WithStack(err)
		}
	} else if err != nil {
		podLog.Error(err, errorFailedToQueryDataIngestEndpointSecret)
		return errors.WithStack(err)
	}

	return nil
}

func (secrets *ingestInitSecrets) createDataIngestSecret(endpointSecretGenerator *dtingestendpoint.EndpointSecretGenerator) error {
	ctx := secrets.ctx
	dynakube := secrets.dynakube
	namesapce := secrets.namespace
	_, err := endpointSecretGenerator.GenerateForNamespace(ctx, dynakube.GetName(), namesapce)

	return errors.WithStack(err)
}
