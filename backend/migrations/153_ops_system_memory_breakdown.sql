-- Ops system metrics: split memory pressure from Linux cache/free breakdown.
-- memory_used_mb now stores pressure usage (total - available) for host metrics.

ALTER TABLE ops_system_metrics
  ADD COLUMN IF NOT EXISTS memory_available_mb BIGINT,
  ADD COLUMN IF NOT EXISTS memory_cache_mb BIGINT,
  ADD COLUMN IF NOT EXISTS memory_free_mb BIGINT,
  ADD COLUMN IF NOT EXISTS memory_raw_used_mb BIGINT;

COMMENT ON COLUMN ops_system_metrics.memory_used_mb IS 'Memory pressure used MB: total - available, excluding reclaimable Linux page cache when host metrics are available.';
COMMENT ON COLUMN ops_system_metrics.memory_usage_percent IS 'Memory pressure percent based on total - available when host metrics are available.';
COMMENT ON COLUMN ops_system_metrics.memory_available_mb IS 'MemAvailable MB from host metrics when available.';
COMMENT ON COLUMN ops_system_metrics.memory_cache_mb IS 'Reclaimable cache MB: cached + buffers + SReclaimable when available.';
COMMENT ON COLUMN ops_system_metrics.memory_free_mb IS 'MemFree MB from host metrics when available.';
COMMENT ON COLUMN ops_system_metrics.memory_raw_used_mb IS 'Raw used MB including cache/buffers where reported by OS library.';
