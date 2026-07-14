CREATE TABLE queue_jobs (
  id TEXT PRIMARY KEY,
  queue TEXT NOT NULL,
  type TEXT NOT NULL,
  payload BLOB NOT NULL,
  status TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  max_retry INTEGER NOT NULL DEFAULT 25,
  timeout_seconds INTEGER NOT NULL DEFAULT 0,
  retention_secs INTEGER NOT NULL DEFAULT 0,
  available_at DATETIME NOT NULL,
  lease_token TEXT NULL,
  leased_until DATETIME NULL,
  last_error TEXT NULL,
  last_failed_at DATETIME NULL,
  completed_at DATETIME NULL,
  expires_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_queue_jobs_reserve ON queue_jobs (queue, status, available_at);
CREATE INDEX idx_queue_jobs_lease ON queue_jobs (status, leased_until);
CREATE INDEX idx_queue_jobs_expiry ON queue_jobs (status, expires_at);

CREATE TABLE queue_locks (
  lock_key TEXT PRIMARY KEY,
  job_id TEXT NOT NULL,
  expires_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_queue_locks_job_id ON queue_locks (job_id);
CREATE INDEX idx_queue_locks_expires_at ON queue_locks (expires_at);

CREATE TABLE queue_stats (
  queue TEXT PRIMARY KEY,
  processed_total INTEGER NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
