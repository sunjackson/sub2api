package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/zeromicro/go-zero/core/collection"
)

func TestProvideTimingWheelService_ReturnsError(t *testing.T) {
	original := newTimingWheel
	t.Cleanup(func() { newTimingWheel = original })

	newTimingWheel = func(_ time.Duration, _ int, _ collection.Execute) (*collection.TimingWheel, error) {
		return nil, errors.New("boom")
	}

	svc, err := ProvideTimingWheelService()
	if err == nil {
		t.Fatalf("期望返回 error，但得到 nil")
	}
	if svc != nil {
		t.Fatalf("期望返回 nil svc，但得到非空")
	}
}

func TestProvideTimingWheelService_Success(t *testing.T) {
	svc, err := ProvideTimingWheelService()
	if err != nil {
		t.Fatalf("期望 err 为 nil，但得到: %v", err)
	}
	if svc == nil {
		t.Fatalf("期望 svc 非空，但得到 nil")
	}
	svc.Stop()
}

func TestProvideAccountQuotaMonitorRunner_RespectsBackgroundWorkerDisable(t *testing.T) {
	t.Setenv("SUB2API_DISABLE_BACKGROUND_WORKERS", "true")
	t.Setenv("SUB2API_ENABLE_ACCOUNT_QUOTA_MONITOR_WORKER", "")

	svc := NewAccountQuotaMonitorService(&quotaBatchRepoStub{}, &quotaBatchAccountRepoStub{}, quotaBatchEncryptorStub{})
	runner := ProvideAccountQuotaMonitorRunner(svc)
	t.Cleanup(runner.Stop)

	if svc.scheduler != nil {
		t.Fatalf("expected scheduler to remain nil while background workers are disabled")
	}
	if runner.started {
		t.Fatalf("expected runner not to start while background workers are disabled")
	}
}

func TestProvideAccountQuotaMonitorRunner_OverrideStartsWhenBackgroundWorkersDisabled(t *testing.T) {
	t.Setenv("SUB2API_DISABLE_BACKGROUND_WORKERS", "true")
	t.Setenv("SUB2API_ENABLE_ACCOUNT_QUOTA_MONITOR_WORKER", "true")

	svc := NewAccountQuotaMonitorService(&quotaBatchRepoStub{}, &quotaBatchAccountRepoStub{}, quotaBatchEncryptorStub{})
	runner := ProvideAccountQuotaMonitorRunner(svc)
	t.Cleanup(runner.Stop)

	if svc.scheduler != runner {
		t.Fatalf("expected quota monitor runner to be installed as scheduler when override is enabled")
	}
	if !runner.started {
		t.Fatalf("expected runner to start when override is enabled")
	}
}

func TestProvideMutatingWorkersRespectDevDisableConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Dev.DisableBackgroundWorkers = true

	accountExpiry := ProvideAccountExpiryService(&sessionWindowMockRepo{}, cfg)
	proxyExpiry := ProvideProxyExpiryService(wireTestProxyRepo{}, cfg)
	subscriptionExpiry := ProvideSubscriptionExpiryService(&subscriptionExpiryRepoStub{}, nil, nil, nil, nil, cfg)
	paymentOrderExpiry := ProvidePaymentOrderExpiryService(&PaymentService{}, nil, nil, cfg)
	channelMonitorSvc := NewChannelMonitorService(nil, nil)
	channelMonitorRunner := ProvideChannelMonitorRunner(channelMonitorSvc, nil, cfg)

	if accountExpiry == nil {
		t.Fatalf("期望 AccountExpiryService 非空")
	}
	if proxyExpiry == nil {
		t.Fatalf("期望 ProxyExpiryService 非空")
	}
	if subscriptionExpiry == nil {
		t.Fatalf("期望 SubscriptionExpiryService 非空")
	}
	if paymentOrderExpiry == nil {
		t.Fatalf("期望 PaymentOrderExpiryService 非空")
	}
	if channelMonitorRunner == nil {
		t.Fatalf("期望 ChannelMonitorRunner 非空")
	}
	if channelMonitorRunner.started {
		t.Fatalf("期望 ChannelMonitorRunner 在禁用后台任务时不启动")
	}
	if channelMonitorSvc.scheduler != nil {
		t.Fatalf("期望 ChannelMonitorService 在禁用后台任务时不注入 scheduler")
	}

	time.Sleep(20 * time.Millisecond)
	accountExpiry.Stop()
	proxyExpiry.Stop()
	subscriptionExpiry.Stop()
	paymentOrderExpiry.Stop()
	channelMonitorRunner.Stop()
}

type wireTestProxyRepo struct{}

func (wireTestProxyRepo) Create(context.Context, *Proxy) error { panic("unexpected Create call") }
func (wireTestProxyRepo) GetByID(context.Context, int64) (*Proxy, error) {
	panic("unexpected GetByID call")
}
func (wireTestProxyRepo) ListByIDs(context.Context, []int64) ([]Proxy, error) {
	panic("unexpected ListByIDs call")
}
func (wireTestProxyRepo) Update(context.Context, *Proxy) error { panic("unexpected Update call") }
func (wireTestProxyRepo) Delete(context.Context, int64) error  { panic("unexpected Delete call") }
func (wireTestProxyRepo) List(context.Context, pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (wireTestProxyRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]Proxy, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (wireTestProxyRepo) ListWithFiltersAndAccountCount(context.Context, pagination.PaginationParams, string, string, string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFiltersAndAccountCount call")
}
func (wireTestProxyRepo) ListActive(context.Context) ([]Proxy, error) {
	panic("unexpected ListActive call")
}
func (wireTestProxyRepo) ListActiveWithAccountCount(context.Context) ([]ProxyWithAccountCount, error) {
	panic("unexpected ListActiveWithAccountCount call")
}
func (wireTestProxyRepo) ExistsByHostPortAuth(context.Context, string, int, string, string) (bool, error) {
	panic("unexpected ExistsByHostPortAuth call")
}
func (wireTestProxyRepo) CountAccountsByProxyID(context.Context, int64) (int64, error) {
	panic("unexpected CountAccountsByProxyID call")
}
func (wireTestProxyRepo) ListAccountSummariesByProxyID(context.Context, int64) ([]ProxyAccountSummary, error) {
	panic("unexpected ListAccountSummariesByProxyID call")
}
func (wireTestProxyRepo) SweepExpiredProxies(context.Context, time.Time) (int64, error) {
	panic("unexpected SweepExpiredProxies call")
}
func (wireTestProxyRepo) ListAllForFallback(context.Context) ([]Proxy, error) {
	panic("unexpected ListAllForFallback call")
}
func (wireTestProxyRepo) CountExpired(context.Context) (int64, error) {
	panic("unexpected CountExpired call")
}
func (wireTestProxyRepo) CountExpiringSoon(context.Context, time.Time) (int64, error) {
	panic("unexpected CountExpiringSoon call")
}
