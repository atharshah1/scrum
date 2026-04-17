package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
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
	if strings.TrimSpace(cfg.IntegrationCryptoKey) == "" {
		log.Error("missing_required_config", "name", "INTEGRATION_CREDENTIALS_KEY")
		os.Exit(1)
	}

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
	wsHub := events.NewWebsocketHub(log, cfg.WebsocketBufferSize, cfg.WebsocketMaxPerOrg, cfg.WebsocketMaxPerUser)
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
	bus.Subscribe("*", automationEngine.Handle)

	notifRepo := notifications.NewRepository(database)
	notifService := notifications.NewService(notifRepo, cfg.NotifyActor)
	bus.Subscribe("issue.created", notifService.HandleEvent)
	bus.Subscribe("issue.updated", notifService.HandleEvent)
	bus.Subscribe("issue.comment_created", notifService.HandleEvent)
	bus.Subscribe("sprint.started", notifService.HandleEvent)
	bus.Subscribe("sprint.completed", notifService.HandleEvent)

	authService := auth.NewService(database, cfg.JWTSecret, cfg.JWTRefreshSecret)
	authHandler := auth.NewHandler(authService, cfg.JWTSecret, cfg.JWTRefreshSecret, sharedCache.RedisClient())
	integrationsHandler, err := integrations.NewHandler(database, authzService, cfg.IntegrationCryptoKey)
	if err != nil {
		log.Error("integrations_handler_failed", "error", err)
		os.Exit(1)
	}

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
	wsRoutes := app.Group("/ws")
	wsRoutes.Get("/", middleware.RateLimitMiddlewareWithKey(40, time.Minute, sharedCache.RedisClient(), func(c *fiber.Ctx) string {
		return websocketRateLimitKey(c, cfg.JWTSecret)
	}), websocket.New(func(conn *websocket.Conn) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("ws_panic_recovered", "panic", recovered, "stack", string(debug.Stack()), "remote_addr", conn.RemoteAddr().String())
				_ = conn.Close()
			}
		}()
		accessToken := extractWebsocketToken(conn)
		if accessToken == "" {
			log.Warn("ws_rejected", "reason", "missing_token", "remote_addr", conn.RemoteAddr().String())
			_ = conn.Close()
			return
		}
		parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		token, err := parser.Parse(accessToken, func(token *jwt.Token) (any, error) {
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			log.Warn("ws_rejected", "reason", "invalid_token", "error", err, "remote_addr", conn.RemoteAddr().String())
			_ = conn.Close()
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			log.Warn("ws_rejected", "reason", "invalid_claims", "remote_addr", conn.RemoteAddr().String())
			_ = conn.Close()
			return
		}
		if tokenType := strings.TrimSpace(asString(claims["type"])); tokenType != "access" {
			log.Warn("ws_rejected", "reason", "invalid_token_type", "token_type", tokenType, "remote_addr", conn.RemoteAddr().String())
			_ = conn.Close()
			return
		}
		orgID, err := uuid.Parse(asString(claims["org_id"]))
		if err != nil {
			log.Warn("ws_rejected", "reason", "invalid_org_claim", "error", err, "remote_addr", conn.RemoteAddr().String())
			_ = conn.Close()
			return
		}
		userID, err := uuid.Parse(asString(claims["sub"]))
		if err != nil {
			log.Warn("ws_rejected", "reason", "invalid_sub_claim", "error", err, "org_id", orgID, "remote_addr", conn.RemoteAddr().String())
			_ = conn.Close()
			return
		}
		expiry, err := parseClaimsExpiry(claims)
		if err != nil {
			log.Warn("ws_rejected", "reason", "invalid_exp_claim", "error", err, "org_id", orgID, "user_id", userID, "remote_addr", conn.RemoteAddr().String())
			_ = conn.Close()
			return
		}
		if !time.Now().Before(expiry) {
			log.Warn("ws_rejected", "reason", "token_expired", "org_id", orgID, "user_id", userID, "remote_addr", conn.RemoteAddr().String())
			_ = conn.Close()
			return
		}
		var projectID *uuid.UUID
		if projectIDParam := strings.TrimSpace(conn.Query("project_id")); projectIDParam != "" {
			id, parseErr := uuid.Parse(projectIDParam)
			if parseErr != nil {
				log.Warn("ws_rejected", "reason", "invalid_project_filter", "error", parseErr, "org_id", orgID, "user_id", userID, "remote_addr", conn.RemoteAddr().String())
				_ = conn.Close()
				return
			}
			projectID = &id
		}
		if err := wsHub.Add(conn, orgID, userID, projectID); err != nil {
			log.Warn("ws_rejected", "reason", "capacity_limit", "error", err, "org_id", orgID, "user_id", userID, "remote_addr", conn.RemoteAddr().String())
			_ = conn.Close()
			return
		}
		log.Info("ws_connected", "org_id", orgID, "user_id", userID, "project_id", projectID, "expires_at", expiry, "remote_addr", conn.RemoteAddr().String())
		defer wsHub.Remove(conn)
		timer := time.AfterFunc(time.Until(expiry), func() {
			log.Info("ws_disconnected", "reason", "token_expired", "org_id", orgID, "user_id", userID, "project_id", projectID)
			// Close is intentionally best-effort; connection may already be closed.
			_ = conn.Close()
		})
		defer timer.Stop()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				log.Info("ws_disconnected", "org_id", orgID, "user_id", userID, "project_id", projectID, "remote_addr", conn.RemoteAddr().String(), "error", err)
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
	releasemodule.NewHandler(database, authzService, bus).RegisterRoutes(secure)
	itsm.NewHandler(database, authzService, bus).RegisterRoutes(secure)
	automation.NewHandler(automationStore, automationEngine).RegisterRoutes(secure)
	webhooks.NewHandler(database, webhookDispatcher, bus).RegisterRoutes(secure)
	integrationsHandler.RegisterRoutes(secure)
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
	automationEngine.Start(ctx)

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

func extractWebsocketToken(conn *websocket.Conn) string {
	if token := strings.TrimSpace(conn.Query("access_token")); token != "" {
		return token
	}
	authHeader := strings.TrimSpace(conn.Headers("Authorization"))
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func extractWebsocketTokenFromContext(c *fiber.Ctx) string {
	if token := strings.TrimSpace(c.Query("access_token")); token != "" {
		return token
	}
	authHeader := strings.TrimSpace(c.Get("Authorization"))
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func websocketRateLimitKey(c *fiber.Ctx, jwtSecret string) string {
	tokenValue := extractWebsocketTokenFromContext(c)
	if tokenValue == "" {
		return "ws:ip:" + c.IP()
	}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	token, err := parser.Parse(tokenValue, func(token *jwt.Token) (any, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return "ws:ip:" + c.IP()
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "ws:ip:" + c.IP()
	}
	if tokenType := strings.TrimSpace(asString(claims["type"])); tokenType != "access" {
		return "ws:ip:" + c.IP()
	}
	orgID := strings.TrimSpace(asString(claims["org_id"]))
	userID := strings.TrimSpace(asString(claims["sub"]))
	if orgID == "" || userID == "" {
		return "ws:ip:" + c.IP()
	}
	return fmt.Sprintf("ws:%s:%s:%s", orgID, userID, c.IP())
}

func parseClaimsExpiry(claims jwt.MapClaims) (time.Time, error) {
	raw, ok := claims["exp"]
	if !ok {
		return time.Time{}, errors.New("exp claim missing")
	}
	switch value := raw.(type) {
	case float64:
		return time.Unix(int64(value), 0).UTC(), nil
	case int64:
		return time.Unix(value, 0).UTC(), nil
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(parsed, 0).UTC(), nil
	default:
		return time.Time{}, errors.New("exp claim has unsupported type")
	}
}
