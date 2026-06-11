package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type accountQuotaMonitorRepository struct {
	db *sql.DB
}

func NewAccountQuotaMonitorRepository(db *sql.DB) service.AccountQuotaMonitorRepository {
	return &accountQuotaMonitorRepository{db: db}
}

func (r *accountQuotaMonitorRepository) Create(ctx context.Context, m *service.AccountQuotaMonitor) error {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO account_quota_monitors (
			name, account_id, provider, endpoint, api_key_override_encrypted,
			enabled, interval_seconds, low_balance_threshold, currency, created_by,
			created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
		RETURNING id, created_at, updated_at
	`, m.Name, m.AccountID, m.Provider, m.Endpoint, nullStringArg(m.APIKeyOverride), m.Enabled,
		m.IntervalSeconds, nullFloatArg(m.LowBalanceThreshold), m.Currency, m.CreatedBy)
	if err := row.Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return fmt.Errorf("create account quota monitor: %w", err)
	}
	return nil
}

func (r *accountQuotaMonitorRepository) GetByID(ctx context.Context, id int64) (*service.AccountQuotaMonitor, error) {
	row := r.db.QueryRowContext(ctx, quotaMonitorSelectSQL()+` WHERE m.id = $1 AND m.deleted_at IS NULL`, id)
	m, err := scanQuotaMonitor(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAccountQuotaMonitorNotFound
		}
		return nil, fmt.Errorf("get account quota monitor: %w", err)
	}
	return m, nil
}

func (r *accountQuotaMonitorRepository) Update(ctx context.Context, m *service.AccountQuotaMonitor) error {
	row := r.db.QueryRowContext(ctx, `
		UPDATE account_quota_monitors
		SET name=$2,
		    account_id=$3,
		    provider=$4,
		    endpoint=$5,
		    api_key_override_encrypted=$6,
		    enabled=$7,
		    interval_seconds=$8,
		    low_balance_threshold=$9,
		    currency=$10,
		    updated_at=NOW()
		WHERE id=$1 AND deleted_at IS NULL
		RETURNING updated_at
	`, m.ID, m.Name, m.AccountID, m.Provider, m.Endpoint, nullStringArg(m.APIKeyOverride),
		m.Enabled, m.IntervalSeconds, nullFloatArg(m.LowBalanceThreshold), m.Currency)
	if err := row.Scan(&m.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrAccountQuotaMonitorNotFound
		}
		return fmt.Errorf("update account quota monitor: %w", err)
	}
	return nil
}

func (r *accountQuotaMonitorRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE account_quota_monitors
		SET deleted_at = NOW(), updated_at = NOW(), enabled = false
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("delete account quota monitor: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return service.ErrAccountQuotaMonitorNotFound
	}
	return nil
}

func (r *accountQuotaMonitorRepository) List(ctx context.Context, params service.AccountQuotaMonitorListParams) ([]*service.AccountQuotaMonitor, int64, error) {
	where, args := quotaMonitorWhere(params)
	countSQL := `SELECT COUNT(*) FROM account_quota_monitors m JOIN accounts a ON a.id = m.account_id ` + where
	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count account quota monitors: %w", err)
	}

	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	args = append(args, pageSize, (page-1)*pageSize)
	query := quotaMonitorSelectSQL() + where + fmt.Sprintf(` ORDER BY m.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list account quota monitors: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items, err := scanQuotaMonitors(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *accountQuotaMonitorRepository) ListEnabled(ctx context.Context) ([]*service.AccountQuotaMonitor, error) {
	rows, err := r.db.QueryContext(ctx, quotaMonitorSelectSQL()+` WHERE m.enabled = true AND m.deleted_at IS NULL ORDER BY m.id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list enabled account quota monitors: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanQuotaMonitors(rows)
}

func (r *accountQuotaMonitorRepository) FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*service.AccountQuotaMonitor, error) {
	out := make(map[int64]*service.AccountQuotaMonitor, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(accountIDs))
	placeholders := make([]string, 0, len(accountIDs))
	for i, id := range accountIDs {
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}
	query := quotaMonitorSelectSQL() + fmt.Sprintf(`
		WHERE m.deleted_at IS NULL AND m.account_id IN (%s)
		ORDER BY m.account_id ASC, m.id DESC
	`, strings.Join(placeholders, ","))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find account quota monitors by accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		m, err := scanQuotaMonitor(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account quota monitor by account: %w", err)
		}
		if _, exists := out[m.AccountID]; !exists {
			out[m.AccountID] = m
		}
	}
	return out, rows.Err()
}

func (r *accountQuotaMonitorRepository) PersistCheckResult(ctx context.Context, result *service.AccountQuotaCheckResult) error {
	if result == nil {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin quota result tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO account_quota_monitor_history (
			monitor_id, account_id, balance, quota_total, quota_used, currency, status, message, checked_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, result.MonitorID, result.AccountID, nullFloatArg(result.Balance), nullFloatArg(result.QuotaTotal),
		nullFloatArg(result.QuotaUsed), result.Currency, result.Status, result.Message, result.CheckedAt); err != nil {
		return fmt.Errorf("insert quota history: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE account_quota_monitors
		SET last_balance=COALESCE($2::numeric, last_balance),
		    last_quota_total=COALESCE($3::numeric, last_quota_total),
		    last_quota_used=COALESCE($4::numeric, last_quota_used),
		    currency=CASE WHEN $2::numeric IS NULL AND $3::numeric IS NULL AND $4::numeric IS NULL THEN currency ELSE $5 END,
		    last_status=$6,
		    last_message=$7,
		    last_checked_at=$8,
		    updated_at=NOW()
		WHERE id=$1 AND deleted_at IS NULL
	`, result.MonitorID, nullFloatArg(result.Balance), nullFloatArg(result.QuotaTotal),
		nullFloatArg(result.QuotaUsed), result.Currency, result.Status, result.Message, result.CheckedAt); err != nil {
		return fmt.Errorf("update quota monitor last result: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit quota result tx: %w", err)
	}
	return nil
}

func (r *accountQuotaMonitorRepository) ListHistory(ctx context.Context, monitorID int64, limit int) ([]*service.AccountQuotaMonitorHistoryEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, monitor_id, account_id, balance, quota_total, quota_used, currency, status, message, checked_at
		FROM account_quota_monitor_history
		WHERE monitor_id = $1
		ORDER BY checked_at DESC
		LIMIT $2
	`, monitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list account quota history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]*service.AccountQuotaMonitorHistoryEntry, 0)
	for rows.Next() {
		entry, err := scanQuotaHistory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (r *accountQuotaMonitorRepository) ComputeMetricsFor(ctx context.Context, ids []int64) (map[int64]service.AccountQuotaMonitorMetrics, error) {
	out := make(map[int64]service.AccountQuotaMonitorMetrics, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids))
	placeholders := make([]string, 0, len(ids))
	for i, id := range ids {
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		out[id] = service.AccountQuotaMonitorMetrics{}
	}
	query := fmt.Sprintf(`
		SELECT monitor_id, balance, checked_at
		FROM account_quota_monitor_history
		WHERE monitor_id IN (%s)
		  AND balance IS NOT NULL
		  AND checked_at >= NOW() - INTERVAL '7 days'
		ORDER BY monitor_id ASC, checked_at ASC
	`, strings.Join(placeholders, ","))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query quota metrics history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	points := map[int64][]quotaMetricPoint{}
	for rows.Next() {
		var id int64
		var balance float64
		var checkedAt time.Time
		if err := rows.Scan(&id, &balance, &checkedAt); err != nil {
			return nil, fmt.Errorf("scan quota metric point: %w", err)
		}
		points[id] = append(points[id], quotaMetricPoint{balance: balance, checkedAt: checkedAt})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for id, pts := range points {
		out[id] = computeQuotaMetrics(pts)
	}
	return out, nil
}

func (r *accountQuotaMonitorRepository) Summary(ctx context.Context) (*service.AccountQuotaMonitorSummary, error) {
	summary := &service.AccountQuotaMonitorSummary{}
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE enabled = true),
		       COUNT(*) FILTER (WHERE last_status = 'ok'),
		       COUNT(*) FILTER (WHERE last_status = 'low_balance'),
		       COUNT(*) FILTER (WHERE last_status = 'error'),
		       COUNT(*) FILTER (WHERE last_status = 'unknown')
		FROM account_quota_monitors
		WHERE deleted_at IS NULL
	`).Scan(&summary.Total, &summary.Enabled, &summary.OK, &summary.LowBalance, &summary.Error, &summary.Unknown); err != nil {
		return nil, fmt.Errorf("query quota summary: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT currency, COALESCE(SUM(last_balance), 0), COUNT(*)
		FROM account_quota_monitors
		WHERE deleted_at IS NULL AND last_balance IS NOT NULL
		GROUP BY currency
		ORDER BY currency
	`)
	if err != nil {
		return nil, fmt.Errorf("query quota summary totals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var total service.AccountQuotaCurrencyTotal
		if err := rows.Scan(&total.Currency, &total.Balance, &total.Count); err != nil {
			return nil, fmt.Errorf("scan quota summary total: %w", err)
		}
		summary.TotalBalanceByCurrency = append(summary.TotalBalanceByCurrency, total)
	}
	return summary, rows.Err()
}

func (r *accountQuotaMonitorRepository) Trend(ctx context.Context, since time.Time, bucket string) ([]*service.AccountQuotaTrendPoint, error) {
	if bucket != "day" {
		bucket = "hour"
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH ranked AS (
		    SELECT date_trunc($2, checked_at) AS bucket,
		           monitor_id,
		           currency,
		           balance,
		           ROW_NUMBER() OVER (
		               PARTITION BY date_trunc($2, checked_at), monitor_id, currency
		               ORDER BY checked_at DESC
		           ) AS rn
		    FROM account_quota_monitor_history
		    WHERE checked_at >= $1 AND balance IS NOT NULL
		)
		SELECT bucket, currency, SUM(balance), COUNT(*)
		FROM ranked
		WHERE rn = 1
		GROUP BY bucket, currency
		ORDER BY bucket ASC, currency ASC
	`, since, bucket)
	if err != nil {
		return nil, fmt.Errorf("query quota trend: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]*service.AccountQuotaTrendPoint, 0)
	for rows.Next() {
		p := &service.AccountQuotaTrendPoint{}
		if err := rows.Scan(&p.Bucket, &p.Currency, &p.Balance, &p.Count); err != nil {
			return nil, fmt.Errorf("scan quota trend point: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func quotaMonitorSelectSQL() string {
	return `
		SELECT m.id, m.name, m.account_id, COALESCE(a.name, ''), COALESCE(a.platform, ''), COALESCE(a.type, ''),
		       m.provider, m.endpoint, m.api_key_override_encrypted, m.enabled, m.interval_seconds,
		       m.low_balance_threshold, m.currency, m.last_balance, m.last_quota_total, m.last_quota_used,
		       m.last_checked_at, m.last_status, m.last_message, m.created_by, m.created_at, m.updated_at
		FROM account_quota_monitors m
		JOIN accounts a ON a.id = m.account_id
	`
}

func quotaMonitorWhere(params service.AccountQuotaMonitorListParams) (string, []any) {
	clauses := []string{"m.deleted_at IS NULL"}
	args := make([]any, 0, 5)
	if params.Provider != "" {
		args = append(args, params.Provider)
		clauses = append(clauses, fmt.Sprintf("m.provider = $%d", len(args)))
	}
	if params.Enabled != nil {
		args = append(args, *params.Enabled)
		clauses = append(clauses, fmt.Sprintf("m.enabled = $%d", len(args)))
	}
	if params.AccountID > 0 {
		args = append(args, params.AccountID)
		clauses = append(clauses, fmt.Sprintf("m.account_id = $%d", len(args)))
	}
	if search := strings.TrimSpace(params.Search); search != "" {
		args = append(args, "%"+search+"%")
		idx := len(args)
		clauses = append(clauses, fmt.Sprintf("(m.name ILIKE $%d OR a.name ILIKE $%d OR m.endpoint ILIKE $%d)", idx, idx, idx))
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

type quotaMonitorScanner interface{ Scan(...any) error }

func scanQuotaMonitor(scanner quotaMonitorScanner) (*service.AccountQuotaMonitor, error) {
	m := &service.AccountQuotaMonitor{}
	var key sql.NullString
	var threshold, balance, total, used sql.NullFloat64
	var checked sql.NullTime
	if err := scanner.Scan(
		&m.ID, &m.Name, &m.AccountID, &m.AccountName, &m.AccountPlatform, &m.AccountType,
		&m.Provider, &m.Endpoint, &key, &m.Enabled, &m.IntervalSeconds,
		&threshold, &m.Currency, &balance, &total, &used,
		&checked, &m.LastStatus, &m.LastMessage, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if key.Valid && strings.TrimSpace(key.String) != "" {
		m.APIKeyOverride = key.String
		m.APIKeyOverrideSet = true
	}
	assignNullFloat(&m.LowBalanceThreshold, threshold)
	assignNullFloat(&m.LastBalance, balance)
	assignNullFloat(&m.LastQuotaTotal, total)
	assignNullFloat(&m.LastQuotaUsed, used)
	if checked.Valid {
		m.LastCheckedAt = &checked.Time
	}
	return m, nil
}

func scanQuotaMonitors(rows *sql.Rows) ([]*service.AccountQuotaMonitor, error) {
	out := make([]*service.AccountQuotaMonitor, 0)
	for rows.Next() {
		m, err := scanQuotaMonitor(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account quota monitor: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanQuotaHistory(scanner quotaMonitorScanner) (*service.AccountQuotaMonitorHistoryEntry, error) {
	entry := &service.AccountQuotaMonitorHistoryEntry{}
	var balance, total, used sql.NullFloat64
	if err := scanner.Scan(&entry.ID, &entry.MonitorID, &entry.AccountID, &balance, &total, &used,
		&entry.Currency, &entry.Status, &entry.Message, &entry.CheckedAt); err != nil {
		return nil, fmt.Errorf("scan account quota history: %w", err)
	}
	assignNullFloat(&entry.Balance, balance)
	assignNullFloat(&entry.QuotaTotal, total)
	assignNullFloat(&entry.QuotaUsed, used)
	return entry, nil
}

func assignNullFloat(dst **float64, n sql.NullFloat64) {
	if !n.Valid {
		return
	}
	v := n.Float64
	*dst = &v
}

func nullFloatArg(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullStringArg(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

type quotaMetricPoint struct {
	balance   float64
	checkedAt time.Time
}

func computeQuotaMetrics(points []quotaMetricPoint) service.AccountQuotaMonitorMetrics {
	var metrics service.AccountQuotaMonitorMetrics
	if len(points) < 2 {
		return metrics
	}
	latest := points[len(points)-1]

	oldest24 := latest
	cutoff24 := latest.checkedAt.Add(-24 * time.Hour)
	for _, p := range points {
		if !p.checkedAt.Before(cutoff24) {
			oldest24 = p
			break
		}
	}
	if consumption := oldest24.balance - latest.balance; consumption > 0 {
		metrics.Consumption24h = &consumption
	}

	oldest := points[0]
	spanHours := latest.checkedAt.Sub(oldest.checkedAt).Hours()
	if spanHours <= 0 {
		return metrics
	}
	consumption := oldest.balance - latest.balance
	if consumption <= 0 {
		return metrics
	}
	avgDaily := consumption / (spanHours / 24.0)
	metrics.AvgDailyConsumption = &avgDaily
	if avgDaily > 0 && latest.balance > 0 {
		days := latest.balance / avgDaily
		metrics.EstimatedDaysRemaining = &days
		depletedAt := latest.checkedAt.Add(time.Duration(days * float64(24*time.Hour)))
		metrics.EstimatedDepletedAt = &depletedAt
	}
	return metrics
}
