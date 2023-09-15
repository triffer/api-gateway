/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package operator

import (
	"context"
	"github.com/kyma-project/api-gateway/apis/operator/v1alpha1"
	"github.com/kyma-project/api-gateway/controllers"
	"github.com/kyma-project/api-gateway/internal/reconciliations/gateway"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func NewAPIGatewayReconciler(mgr manager.Manager) *APIGatewayReconciler {
	return &APIGatewayReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		log:           mgr.GetLogger().WithName("apigateway-controller"),
		statusHandler: controllers.NewStatusHandler(mgr.GetClient()),
	}
}

//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=apigateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=apigateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=apigateways/finalizers,verbs=update
//+kubebuilder:rbac:groups=networking.istio.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete

func (r *APIGatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	apiGatewayCR := v1alpha1.APIGateway{}
	if err := r.Client.Get(ctx, req.NamespacedName, &apiGatewayCR); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("Skipped reconciliation, because ApiGateway CR was not found")
			return ctrl.Result{}, nil
		}
		r.log.Info("Could not get APIGateway CR")
		return ctrl.Result{}, err
	}

	if kymaGatewayErr := gateway.ReconcileKymaGateway(ctx, r.Client, apiGatewayCR); kymaGatewayErr != nil {
		return r.requeueReconciliation(ctx, apiGatewayCR, kymaGatewayErr, "Reconciliation Kyma Gateway failed")
	}

	return r.finishReconcile(ctx, apiGatewayCR)
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIGatewayReconciler) SetupWithManager(mgr ctrl.Manager, c controllers.RateLimiterConfig) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.APIGateway{}).
		WithOptions(controller.Options{
			RateLimiter: controllers.NewRateLimiter(c),
		}).
		Complete(r)
}

// requeueReconciliation cancels the reconciliation and requeues the request.
func (r *APIGatewayReconciler) requeueReconciliation(ctx context.Context, cr v1alpha1.APIGateway, err error, description string) (ctrl.Result, error) {
	r.log.Error(err, "Reconcile failed")

	statusUpdateErr := r.statusHandler.UpdateToError(ctx, &cr, description)
	if statusUpdateErr != nil {
		r.log.Error(statusUpdateErr, "Error during updating status to error")
	}

	return ctrl.Result{}, err
}

func (r *APIGatewayReconciler) finishReconcile(ctx context.Context, cr v1alpha1.APIGateway) (ctrl.Result, error) {

	if err := r.statusHandler.UpdateToReady(ctx, &cr); err != nil {
		r.log.Error(err, "Update status to ready failed")
		return ctrl.Result{}, err
	}

	r.log.Info("Reconciled status successfully")

	return ctrl.Result{}, nil
}