-- Record non-payment recharge sources for affiliate rebate audit display.
-- For direct balance redeem codes, source_ref stores the redeem code, which is
-- the business order number shown in affiliate rebate records.

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_ref VARCHAR(128) NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_amount DECIMAL(20,8) NULL;

COMMENT ON COLUMN user_affiliate_ledger.source_ref IS 'Non-payment source reference for affiliate rebates, e.g. balance redeem code';
COMMENT ON COLUMN user_affiliate_ledger.source_amount IS 'Original recharge amount for non-payment affiliate rebate sources';

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_source_ref
    ON user_affiliate_ledger(source_ref)
    WHERE source_ref IS NOT NULL;

-- Best-effort historical backfill for direct balance redeem rebates created
-- before source_ref existed. Only one-to-one matches are updated.
WITH candidates AS (
    SELECT ual.id AS ledger_id,
           rc.code AS redeem_code,
           rc.value AS redeem_amount,
           COUNT(*) OVER (PARTITION BY ual.id) AS ledger_match_count,
           COUNT(*) OVER (PARTITION BY rc.id) AS redeem_match_count,
           ROW_NUMBER() OVER (
               PARTITION BY ual.id
               ORDER BY ABS(EXTRACT(EPOCH FROM (ual.created_at - rc.used_at))), rc.id
           ) AS ledger_rank
    FROM user_affiliate_ledger ual
    JOIN user_affiliates invitee_aff
      ON invitee_aff.user_id = ual.source_user_id
     AND invitee_aff.inviter_id = ual.user_id
    JOIN redeem_codes rc
      ON rc.used_by = ual.source_user_id
     AND rc.status = 'used'
     AND rc.type = 'balance'
     AND rc.value > 0
     AND rc.used_at IS NOT NULL
     AND ual.created_at BETWEEN rc.used_at - INTERVAL '10 minutes'
                            AND rc.used_at + INTERVAL '10 minutes'
    WHERE ual.action = 'accrue'
      AND ual.source_order_id IS NULL
      AND ual.source_ref IS NULL
)
UPDATE user_affiliate_ledger ual
SET source_ref = candidates.redeem_code,
    source_amount = candidates.redeem_amount,
    updated_at = NOW()
FROM candidates
WHERE ual.id = candidates.ledger_id
  AND candidates.ledger_match_count = 1
  AND candidates.redeem_match_count = 1
  AND candidates.ledger_rank = 1;
