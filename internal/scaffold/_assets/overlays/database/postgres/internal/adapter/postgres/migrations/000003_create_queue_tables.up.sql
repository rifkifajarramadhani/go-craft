CREATE TABLE queue_jobs (
  id VARCHAR(255) PRIMARY KEY,
  queue VARCHAR(100) NOT NULL,
  type VARCHAR(255) NOT NULL,
  payload JSONB NOT NULL,
  status VARCHAR(20) NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  max_retry INTEGER NOT NULL DEFAULT 25,
  timeout_seconds INTEGER NOT NULL DEFAULT 0,
  retention_secs INTEGER NOT NULL DEFAULT 0,
  available_at TIMESTAMPTZ NOT NULL,
  lease_token VARCHAR(36) NULL,
  leased_until TIMESTAMPTZ NULL,
  last_error TEXT NULL,
  last_failed_at TIMESTAMPTZ NULL,
  completed_at TIMESTAMPTZ NULL,
  expires_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_queue_jobs_reserve ON queue_jobs (queue, status, available_at);
CREATE INDEX idx_queue_jobs_lease ON queue_jobs (status, leased_until);
CREATE INDEX idx_queue_jobs_expiry ON queue_jobs (status, expires_at);

CREATE TABLE queue_locks (
  lock_key VARCHAR(191) PRIMARY KEY,
  job_id VARCHAR(255) NOT NULL,
  expires_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_queue_locks_job_id ON queue_locks (job_id);
CREATE INDEX idx_queue_locks_expires_at ON queue_locks (expires_at);

CREATE TABLE queue_stats (
  queue VARCHAR(100) PRIMARY KEY,
  processed_total BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
