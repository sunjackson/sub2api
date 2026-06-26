//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestValidateOrderInputRejectsSubscriptionPurchases(t *testing.T) {
	t.Parallel()

	svc := &PaymentService{}
	plan, err := svc.validateOrderInput(context.Background(), CreateOrderRequest{
		OrderType: payment.OrderTypeSubscription,
		PlanID:    7,
	}, &PaymentConfig{})

	require.Nil(t, plan)
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, infraerrors.Code(err))
	require.Equal(t, "SUBSCRIPTION_PURCHASE_DISABLED", infraerrors.Reason(err))
}
