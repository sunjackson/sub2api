package service

import (
	"errors"
	"testing"
	"time"

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
