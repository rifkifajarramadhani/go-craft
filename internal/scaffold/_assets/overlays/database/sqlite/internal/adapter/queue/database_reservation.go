package queueinfra

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func reserveDatabaseJob(ctx context.Context, db *gorm.DB, queueName string, now time.Time, reservation time.Duration) (*databaseJob, error) {
	token := uuid.NewString()
	leasedUntil := now.Add(reservation)

	var job databaseJob

	result := db.WithContext(ctx).Raw(`
		UPDATE queue_jobs
		SET status = ?, attempts = attempts + 1, lease_token = ?, leased_until = ?, updated_at = ?
		WHERE id = (
			SELECT id FROM queue_jobs
			WHERE queue = ?
			  AND ((status IN (?, ?) AND available_at <= ?) OR (status = ? AND leased_until <= ?))
			ORDER BY available_at ASC, created_at ASC
			LIMIT 1
		)
		RETURNING *`,
		jobStatusActive, token, leasedUntil, now, queueName,
		jobStatusPending, jobStatusRetry, now, jobStatusActive, now,
	).Scan(&job)
	if result.Error != nil {
		return nil, result.Error
	}

	if result.RowsAffected == 0 || job.ID == "" {
		return nil, nil
	}

	return &job, nil
}
