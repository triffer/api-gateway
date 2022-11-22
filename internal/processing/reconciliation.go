package processing

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	gatewayv1beta1 "github.com/kyma-incubator/api-gateway/api/v1beta1"
	"github.com/kyma-incubator/api-gateway/internal/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GenericReconciler struct {
	ReconciliationCommand
	logger     *logr.Logger
	ctx        context.Context
	client     client.Client
	processors []ReconciliationProcessor
}

func NewGenericReconciler(command ReconciliationCommand, logger logr.Logger, ctx context.Context, client client.Client, processors []ReconciliationProcessor) GenericReconciler {
	return GenericReconciler{
		logger:                &logger,
		processors:            processors,
		ctx:                   ctx,
		client:                client,
		ReconciliationCommand: command,
	}
}

type ReconciliationCommand interface {
	Validate(*gatewayv1beta1.APIRule) ([]validation.Failure, error)
}

type ReconciliationProcessor interface {
	EvaluateReconciliation(*gatewayv1beta1.APIRule) ([]*ObjectChange, error)
}

// Reconcile executes the reconciliation of the APIRule using the given reconciliation command.
func (g GenericReconciler) Reconcile(apiRule *gatewayv1beta1.APIRule) ReconciliationStatus {

	validationFailures, err := g.Validate(apiRule)
	if err != nil {
		// We set the status to skipped because it was not the validation that failed, but an error occurred during validation.
		return GetStatusForError(g.logger, err, gatewayv1beta1.StatusSkipped)
	}

	if len(validationFailures) > 0 {
		failuresJson, _ := json.Marshal(validationFailures)
		g.logger.Info(fmt.Sprintf(`Validation failure {"controller": "Api", "request": "%s/%s", "failures": %s}`, apiRule.Namespace, apiRule.Name, string(failuresJson)))
		return getFailedValidationStatus(validationFailures)
	}

	for _, processor := range g.processors {

		objectChanges, err := processor.EvaluateReconciliation(apiRule)
		if err != nil {
			return GetStatusForError(g.logger, err, gatewayv1beta1.StatusSkipped)
		}

		err = applyChanges(g.ctx, g.client, objectChanges...)
		if err != nil {
			//  "We don't know exactly which object(s) are not updated properly. The safest approach is to assume nothing is correct and just use `StatusError`."
			return GetStatusForError(g.logger, err, gatewayv1beta1.StatusError)
		}
	}

	return getOkStatus()
}

// applyChanges applies the given commands on the cluster
func applyChanges(ctx context.Context, client client.Client, changes ...*ObjectChange) error {

	for _, change := range changes {
		err := applyChange(ctx, client, change)
		if err != nil {
			return err
		}
	}

	return nil
}

func applyChange(ctx context.Context, client client.Client, change *ObjectChange) error {
	var err error

	switch change.Action {
	case "create":
		err = client.Create(ctx, change.Obj)
	case "update":
		err = client.Update(ctx, change.Obj)
	case "delete":
		err = client.Delete(ctx, change.Obj)
	default:
		err = fmt.Errorf("apply action %s is not supported", change.Action)
	}

	if err != nil {
		return err
	}

	return nil
}
