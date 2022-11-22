package istio

import (
	gatewayv1beta1 "github.com/kyma-incubator/api-gateway/api/v1beta1"
	"github.com/kyma-incubator/api-gateway/internal/processing"
	"github.com/kyma-incubator/api-gateway/internal/validation"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
)

type Reconciliation struct {
	config *processing.ReconciliationConfig
}

func NewIstioReconciliation(config processing.ReconciliationConfig) processing.GenericReconciler {
	vsProcessor := NewVirtualService(config)

	cmd := Reconciliation{
		config: &config,
	}

	return processing.NewGenericReconciler(cmd, config.Logger, config.Ctx, config.Client, []processing.ReconciliationProcessor{vsProcessor})
}

func (r Reconciliation) Validate(apiRule *gatewayv1beta1.APIRule) ([]validation.Failure, error) {

	var vsList networkingv1beta1.VirtualServiceList
	if err := r.config.Client.List(r.config.Ctx, &vsList); err != nil {
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
