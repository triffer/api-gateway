package ory

import (
	"context"

	gatewayv1beta1 "github.com/kyma-incubator/api-gateway/api/v1beta1"
	"github.com/kyma-incubator/api-gateway/internal/processing"
	"github.com/kyma-incubator/api-gateway/internal/validation"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciliation struct {
	config processing.ReconciliationConfig
}

func NewOryReconciliation(config processing.ReconciliationConfig) Reconciliation {
	return Reconciliation{
		config: config,
	}
}

func (r Reconciliation) Validate(ctx context.Context, client client.Client, apiRule *gatewayv1beta1.APIRule) ([]validation.Failure, error) {
	var vsList networkingv1beta1.VirtualServiceList
	if err := client.List(ctx, &vsList); err != nil {
		return make([]validation.Failure, 0), err
	}

	validator := validation.APIRule{
		JwtValidator:      &jwtValidator{},
		ServiceBlockList:  r.config.ServiceBlockList,
		DomainAllowList:   r.config.DomainAllowList,
		HostBlockList:     r.config.HostBlockList,
		DefaultDomainName: r.config.DefaultDomainName,
	}
	return validator.Validate(apiRule, vsList), nil
}

func (r Reconciliation) Evaluate(ctx context.Context, client client.Client, apiRule *gatewayv1beta1.APIRule) error {
	vsProcessor := NewVirtualServiceProcessor(r.config)
	acProcessor := NewAccessRuleProcessor(r.config)

	processors := []processing.ReconciliationProcessor{vsProcessor, acProcessor}

	return processing.Evaluate(ctx, client, apiRule, processors)
}
