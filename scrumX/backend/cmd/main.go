package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/configs"
	"github.com/atharshah1/scrum/scrumX/backend/internal/ai"
	"github.com/atharshah1/scrum/scrumX/backend/internal/apidocs"
	"github.com/atharshah1/scrum/scrumX/backend/internal/auth"
	"github.com/atharshah1/scrum/scrumX/backend/internal/authz"
	"github.com/atharshah1/scrum/scrumX/backend/internal/automation"
	"github.com/atharshah1/scrum/scrumX/backend/internal/boards"
	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/atharshah1/scrum/scrumX/backend/internal/insights"
	"github.com/atharshah1/scrum/scrumX/backend/internal/integrations"
	"github.com/atharshah1/scrum/scrumX/backend/internal/issues"
	"github.com/atharshah1/scrum/scrumX/backend/internal/itsm"
	"github.com/atharshah1/scrum/scrumX/backend/internal/notifications"
	"github.com/atharshah1/scrum/scrumX/backend/internal/organizations"
	"github.com/atharshah1/scrum/scrumX/backend/internal/projects"
	"github.com/atharshah1/scrum/scrumX/backend/internal/rbac"
	releasemodule "github.com/atharshah1/scrum/scrumX/backend/internal/release"
	"github.com/atharshah1/scrum/scrumX/backend/internal/sprints"
	timetracking "github.com/atharshah1/scrum/scrumX/backend/internal/time"
	"github.com/atharshah1/scrum/scrumX/backend/internal/users"
	"github.com/atharshah1/scrum/scrumX/backend/internal/webhooks"
	"github.com/atharshah1/scrum/scrumX/backend/internal/workflows"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/cache"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/db"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/logger"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/middleware"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/observability"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func main() {
	cfg := configs.Load()
	log := logger.New()

	database, err := db.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Error("db_connection_failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	internalBus := events.NewInternalBus()
	var kafkaPublisher *events.KafkaEventPublisher
	var outboxStore *events.OutboxStore
	if cfg.KafkaEnabled {
		kafkaPublisher = events.NewKafkaEventPublisher(cfg.KafkaBrokers, cfg.AutomationKafkaTopic, cfg.KafkaBatchTimeout)
		if kafkaPublisher == nil {
			log.Warn("kafka_enabled_but_not_configured", "brokers", cfg.KafkaBrokers, "topic", cfg.AutomationKafkaTopic)
		} else {
			outboxStore = events.NewOutboxStore(database)
		}
	}
	bus := events.NewBus(log, internalBus, kafkaPublisher, outboxStore)
	if kafkaPublisher != nil {
		defer kafkaPublisher.Close()
	}
	wsHub := events.NewWebsocketHub(log, cfg.WebsocketBufferSize)
	bus.Subscribe("*", wsHub.Broadcast)

	sharedCache := cache.NewTTLCache(cfg.CacheTTL)
	if cfg.RedisEnabled {
		sharedCache = cache.NewRedisBackedTTLCache(cfg.CacheTTL, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	}
	defer sharedCache.Close()
	metrics := observability.NewMetrics()

	authzService := authz.NewService(database)
	issueRepo := issues.NewRepository(database)
	issueService := issues.NewService(issueRepo, bus, authzService, sharedCache)
	aiService := ai.NewService(ai.NewLLMClient(cfg.AIProvider, cfg.AIAPIKey), sharedCache, cfg.AITimeout)

	webhookDispatcher := webhooks.NewDispatcher(log, bus, cfg.WebhookTimeout, database)
	automationStore := automation.NewStore(database)
	automationEngine := automation.NewEngine(log, automationStore, webhookDispatcher, cfg.AutomationWorkers, issueService, cfg.AutomationMaxRetries, cfg.AutomationBackoff)
	if !cfg.KafkaEnabled {
		bus.Subscribe("*", automationEngine.Enqueue)
	}

	notifRepo := notifications.NewRepository(database)
	notifService := notifications.NewService(notifRepo, cfg.NotifyActor)
	bus.Subscribe("issue.created", notifService.HandleEvent)
	bus.Subscribe("issue.updated", notifService.HandleEvent)
	bus.Subscribe("issue.comment_created", notifService.HandleEvent)
	bus.Subscribe("sprint.started", notifService.HandleEvent)
	bus.Subscribe("sprint.completed", notifService.HandleEvent)

	authService := auth.NewService(database, cfg.JWTSecret, cfg.JWTRefreshSecret)
	authHandler := auth.NewHandler(authService, cfg.JWTSecret, cfg.JWTRefreshSecret, sharedCache.RedisClient())

	app := fiber.New(fiber.Config{BodyLimit: 1024 * 1024})
	app.Use(middleware.LoggingMiddleware(log))
	app.Use(middleware.MetricsMiddleware(metrics))

	app.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	app.Get("/openapi.yaml", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/yaml")
		return c.Send(apidocs.OpenAPI)
	})
	app.Get("/health/ready", func(c *fiber.Ctx) error {
		if cfg.KafkaEnabled && kafkaPublisher != nil {
			ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
			defer cancel()
			if err := kafkaPublisher.Ping(ctx); err != nil {
				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
					"status": "degraded",
					"kafka":  "unreachable",
				})
			}
		}
		if cfg.RedisEnabled {
			ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
			defer cancel()
			if err := sharedCache.Ping(ctx); err != nil {
				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
					"status": "degraded",
					"redis":  "unreachable",
				})
			}
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})
	app.Get("/metrics", middleware.ProtectMetrics(cfg.MetricsToken), func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/plain; version=0.0.4")
		return c.SendString(metrics.PrometheusText())
	})
	app.Get("/ws", websocket.New(func(conn *websocket.Conn) {
		accessToken := strings.TrimSpace(conn.Query("token"))
		if accessToken == "" {
			log.Warn("ws_rejected", "reason", "missing token")
			_ = conn.Close()
			return
		}
		parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		token, err := parser.Parse(accessToken, func(token *jwt.Token) (any, error) {
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			log.Warn("ws_rejected", "reason", "invalid token", "error", err)
			_ = conn.Close()
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			log.Warn("ws_rejected", "reason", "invalid claims")
			_ = conn.Close()
			return
		}
		orgID, err := uuid.Parse(asString(claims["org_id"]))
		if err != nil {
			log.Warn("ws_rejected", "reason", "invalid org claim", "error", err)
			_ = conn.Close()
			return
		}
		var projectID *uuid.UUID
		if projectIDParam := strings.TrimSpace(conn.Query("project_id")); projectIDParam != "" {
			id, parseErr := uuid.Parse(projectIDParam)
			if parseErr != nil {
				log.Warn("ws_rejected", "reason", "invalid project filter", "error", parseErr)
				_ = conn.Close()
				return
			}
			projectID = &id
		}
		wsHub.Add(conn, orgID, projectID)
		defer wsHub.Remove(conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))

	api := app.Group("/api/v1")
	authHandler.RegisterRoutes(api)

	secure := api.Group("", middleware.RateLimitMiddleware(300, time.Minute, sharedCache.RedisClient()), middleware.AuthMiddleware(cfg.JWTSecret), middleware.OrgContextMiddleware())
	secure.Use(middleware.RBACMiddleware("Admin", "Member", "Viewer"))
	secure.Use(middleware.AuditMiddleware(database))

	issues.NewHandler(issueService, sharedCache).RegisterRoutes(secure)
	ai.NewHandler(
		aiService,
		issueService,
		cfg.AIFromTextEnabled,
		cfg.AIFromTextLimitPerMin,
		cfg.AIFromTextDailyQuota,
		cfg.AIFromTextMaxChars,
	).RegisterRoutes(secure)
	organizations.NewHandler(database, authzService).RegisterRoutes(secure)
	users.NewHandler(database, authzService).RegisterRoutes(secure)
	projects.NewHandler(database, authzService).RegisterRoutes(secure)
	sprints.NewHandler(database, bus, authzService, sharedCache).RegisterRoutes(secure)
	boards.NewHandler(database, authzService, sharedCache).RegisterRoutes(secure)
	timetracking.NewHandler().RegisterRoutes(secure)
	releasemodule.NewHandler(database, authzService).RegisterRoutes(secure)
	itsm.NewHandler(database, authzService).RegisterRoutes(secure)
	automation.NewHandler(automationStore, automationEngine).RegisterRoutes(secure)
	webhooks.NewHandler(webhookDispatcher, bus).RegisterRoutes(secure)
	integrations.NewHandler(database, authzService).RegisterRoutes(secure)
	insights.NewHandler(database).RegisterRoutes(secure)
	workflows.NewHandler(database, authzService).RegisterRoutes(secure)
	notifications.NewHandler(notifRepo).RegisterRoutes(secure)
	rbac.NewHandler(database, authzService).RegisterRoutes(secure)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if cfg.KafkaEnabled && kafkaPublisher != nil && outboxStore != nil {
		dispatcher := events.NewOutboxDispatcher(log, outboxStore, kafkaPublisher, 100, 500*time.Millisecond, 5, 250*time.Millisecond)
		go dispatcher.Start(ctx)
	}
	if !cfg.KafkaEnabled {
		automationEngine.Start(ctx)
	}

	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Error("server_failed", slog.Any("error", err))
			stop()
		}
	}()

	<-ctx.Done()
	_ = app.Shutdown()
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}
