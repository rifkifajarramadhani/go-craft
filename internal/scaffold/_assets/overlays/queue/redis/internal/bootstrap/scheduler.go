package bootstrap

import (
	"time"

	"__GO_CRAFT_MODULE__/internal/adapter/cron"
	"__GO_CRAFT_MODULE__/internal/adapter/jobs"
	"__GO_CRAFT_MODULE__/internal/config"
	"__GO_CRAFT_MODULE__/internal/queue"
	"__GO_CRAFT_MODULE__/internal/scheduler"
)

func ScheduleRegistry(cfg *config.Config) (*scheduler.Registry, error) {
	registry, err := scheduler.NewRegistry(cfg.Scheduler.Timezone, cron.Parser{})
	if err != nil {
		return nil, err
	}

	err = registry.Register(scheduler.Definition{
		Name:     "cleanup-refresh-tokens",
		Cron:     "0 0 * * *",
		Timezone: cfg.Scheduler.Timezone,
		Job:      func() queue.Job { return jobs.CleanupRefreshTokens{} },
		DispatchOptions: queue.DispatchOptions{
			Queue: "maintenance", MaxRetry: 3, Retention: 2 * time.Minute,
		},
	})

	return registry, err
}
