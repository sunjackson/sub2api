//go:build unit

package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/authidentity"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type registerSettingRepoStub struct {
	values map[string]string
}

func (s *registerSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *registerSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", service.ErrSettingNotFound
}

func (s *registerSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *registerSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := s.values[key]; ok {
			result[key] = v
		}
	}
	return result, nil
}

func (s *registerSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *registerSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *registerSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type failingUseRedeemRepo struct {
	service.RedeemCodeRepository
	client *dbent.Client
	err    error
}

func (r *failingUseRedeemRepo) GetByCode(ctx context.Context, code string) (*service.RedeemCode, error) {
	entity, err := r.client.RedeemCode.Query().
		Where(redeemcode.CodeEQ(code)).
		Only(ctx)
	if err != nil {
		return nil, service.ErrRedeemCodeNotFound
	}
	return &service.RedeemCode{
		ID:     entity.ID,
		Code:   entity.Code,
		Type:   entity.Type,
		Status: entity.Status,
		UsedBy: entity.UsedBy,
		UsedAt: entity.UsedAt,
	}, nil
}

func (r *failingUseRedeemRepo) Use(context.Context, int64, int64) error {
	return r.err
}

type registerTransactionUserRepo struct {
	service.UserRepository
}

func (r *registerTransactionUserRepo) ExistsByEmail(context.Context, string) (bool, error) {
	return false, nil
}

func newRegisterTransactionAuthService(t *testing.T, redeemRepoFactory func(*dbent.Client) service.RedeemCodeRepository) (*service.AuthService, *dbent.Client) {
	t.Helper()

	sqlDB, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	_, err = sqlDB.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	_, err = sqlDB.Exec(`
CREATE TABLE IF NOT EXISTS user_provider_default_grants (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	provider_type TEXT NOT NULL,
	grant_reason TEXT NOT NULL DEFAULT 'first_bind',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, provider_type, grant_reason)
)`)
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, sqlDB)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			ExpireHour: 1,
		},
		Default: config.DefaultConfig{
			UserBalance:     3.5,
			UserConcurrency: 2,
		},
	}
	settingRepo := &registerSettingRepoStub{values: map[string]string{
		service.SettingKeyRegistrationEnabled:   "true",
		service.SettingKeyInvitationCodeEnabled: "true",
	}}
	settingService := service.NewSettingService(settingRepo, cfg)
	userRepo := &registerTransactionUserRepo{
		UserRepository: repository.NewUserRepository(client, sqlDB),
	}
	redeemRepo := redeemRepoFactory(client)

	return service.NewAuthService(
		client,
		userRepo,
		redeemRepo,
		nil,
		cfg,
		settingService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	), client
}

func TestAuthService_Register_InvitationUseFailureRollsBackUserWithEnt(t *testing.T) {
	ctx := context.Background()
	svc, client := newRegisterTransactionAuthService(t, func(client *dbent.Client) service.RedeemCodeRepository {
		return &failingUseRedeemRepo{client: client, err: service.ErrRedeemCodeUsed}
	})

	_, err := client.RedeemCode.Create().
		SetCode("INVITE-TX").
		SetType(service.RedeemTypeInvitation).
		SetStatus(service.StatusUnused).
		Save(ctx)
	require.NoError(t, err)

	token, createdUser, err := svc.RegisterWithVerification(ctx, "rollback@example.com", "password", "", "", "INVITE-TX", "")
	require.ErrorIs(t, err, service.ErrInvitationCodeInvalid)
	require.Empty(t, token)
	require.Nil(t, createdUser)

	userCount, err := client.User.Query().
		Where(user.EmailEQ("rollback@example.com")).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, userCount)

	storedCode, err := client.RedeemCode.Query().
		Where(redeemcode.CodeEQ("INVITE-TX")).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, service.StatusUnused, storedCode.Status)
	require.Nil(t, storedCode.UsedBy)
}

func TestAuthService_Register_InvitationSuccessCreatesEmailIdentityWithEnt(t *testing.T) {
	ctx := context.Background()
	svc, client := newRegisterTransactionAuthService(t, func(client *dbent.Client) service.RedeemCodeRepository {
		return repository.NewRedeemCodeRepository(client)
	})

	_, err := client.RedeemCode.Create().
		SetCode("INVITE-IDENTITY").
		SetType(service.RedeemTypeInvitation).
		SetStatus(service.StatusUnused).
		Save(ctx)
	require.NoError(t, err)

	token, createdUser, err := svc.RegisterWithVerification(ctx, "invite-identity@example.com", "password", "", "", "INVITE-IDENTITY", "")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotNil(t, createdUser)

	identity, err := client.AuthIdentity.Query().
		Where(
			authidentity.UserIDEQ(createdUser.ID),
			authidentity.ProviderTypeEQ("email"),
			authidentity.ProviderKeyEQ("email"),
			authidentity.ProviderSubjectEQ("invite-identity@example.com"),
		).
		Only(ctx)
	require.NoError(t, err)
	require.NotNil(t, identity.VerifiedAt)

	storedCode, err := client.RedeemCode.Query().
		Where(redeemcode.CodeEQ("INVITE-IDENTITY")).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, service.StatusUsed, storedCode.Status)
	require.NotNil(t, storedCode.UsedBy)
	require.Equal(t, createdUser.ID, *storedCode.UsedBy)
}

func TestAuthService_Register_InvitationUsesNormalizedEmailUniquenessInTx(t *testing.T) {
	ctx := context.Background()
	svc, client := newRegisterTransactionAuthService(t, func(client *dbent.Client) service.RedeemCodeRepository {
		return repository.NewRedeemCodeRepository(client)
	})

	_, err := client.User.Create().
		SetEmail("alias.user@gmail.com").
		SetPasswordHash("hash").
		SetUsername("existing-alias").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.RedeemCode.Create().
		SetCode("INVITE-ALIAS").
		SetType(service.RedeemTypeInvitation).
		SetStatus(service.StatusUnused).
		Save(ctx)
	require.NoError(t, err)

	token, createdUser, err := svc.RegisterWithVerification(ctx, "aliasuser+tag@googlemail.com", "password", "", "", "INVITE-ALIAS", "")
	require.ErrorIs(t, err, service.ErrEmailExists)
	require.Empty(t, token)
	require.Nil(t, createdUser)

	count, err := client.User.Query().
		Where(user.EmailEQ("aliasuser+tag@googlemail.com")).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)

	storedCode, err := client.RedeemCode.Query().
		Where(redeemcode.CodeEQ("INVITE-ALIAS")).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, service.StatusUnused, storedCode.Status)
	require.Nil(t, storedCode.UsedBy)
}

var _ service.RedeemCodeRepository = (*failingUseRedeemRepo)(nil)
