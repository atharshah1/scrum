package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/configs"
	"github.com/atharshah1/scrum/scrumX/backend/internal/authz"
	"github.com/atharshah1/scrum/scrumX/backend/internal/automation"
	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/atharshah1/scrum/scrumX/backend/internal/issues"
	"github.com/atharshah1/scrum/scrumX/backend/internal/webhooks"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/cache"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/db"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/logger"
)

func main() {
	cfg := configs.Load()
	log := logger.New()

	if !cfg.KafkaEnabled {
		log.Warn("worker_exits_kafka_disabled")
		return
	}

	database, err := db.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Error("db_connection_failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	workerBus := events.NewBus(log, events.NewInternalBus(), nil)
	authzService := authz.NewService(database)
	issueService := issues.NewService(issues.NewRepository(database), workerBus, authzService, cache.NewTTLCache(30*time.Second))
	webhookDispatcher := webhooks.NewDispatcher(log, workerBus, cfg.WebhookTimeout, database)
	automationStore := automation.NewStore(database)
	automationEngine := automation.NewEngine(log, automationStore, webhookDispatcher, cfg.AutomationWorkers, issueService, cfg.AutomationMaxRetries, cfg.AutomationBackoff)

	consumer := events.NewKafkaAutomationConsumer(log, cfg.KafkaBrokers, cfg.AutomationKafkaTopic, cfg.AutomationKafkaGroup, automationEngine.Enqueue)
	if consumer == nil {
		log.Error("worker_kafka_consumer_init_failed", slog.String("brokers", cfg.KafkaBrokers), slog.String("topic", cfg.AutomationKafkaTopic), slog.String("group", cfg.AutomationKafkaGroup))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	automationEngine.Start(ctx)

	if err := consumer.Start(ctx); err != nil {
		log.Error("worker_consumer_failed", "error", err)
		os.Exit(1)
	}
}
