package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/http/handler"
	"github.com/Abdibaye/cafnet_backend/internal/http/router"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/service"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 1. Load centralized configuration
	cfg, err := config.LoadServerConfig()
	if err != nil {
		logger.Error("configuration loading failed", "error", err)
		os.Exit(1)
	}

	logger.Info("starting cafnet http api server",
		"environment", cfg.Environment,
		"addr", cfg.HTTPAddr,
		"allowed_origins", cfg.AllowedOrigins,
	)

	// 2. Connect to Neon PostgreSQL
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelConnect()

	pool, err := db.Connect(connectCtx, cfg.DB)
	if err != nil {
		logger.Error("failed to connect to Neon PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 3. Initialize Repositories & Transaction Manager
	txMgr := repository.NewTxManager(pool)
	userRepo := repository.NewUserRepository(pool)
	businessRepo := repository.NewBusinessRepository(pool)
	ownershipRepo := repository.NewOwnershipRepository(pool)
	claimRepo := repository.NewClaimRepository(pool)
	subRepo := repository.NewSubmissionRepository(pool)
	menuRepo := repository.NewMenuRepository(pool)
	updateRepo := repository.NewUpdateRepository(pool)
	reservationRepo := repository.NewReservationRepository(pool)
	reviewRepo := repository.NewReviewRepository(pool)
	reportRepo := repository.NewReportRepository(pool)
	customerListRepo := repository.NewCustomerListRepository(pool)
	adminActionRepo := repository.NewAdminActionRepository(pool)
	sessionRepo := repository.NewSessionRepository(pool)
	otpRepo := repository.NewOTPRepository(pool)

	// 4. Initialize Authorization & Ownership Workflow Services
	authzService := service.NewAuthorizationService(ownershipRepo, businessRepo)
	ownershipWorkflow := service.NewOwnershipWorkflowService(
		authzService,
		claimRepo,
		subRepo,
		businessRepo,
		ownershipRepo,
		adminActionRepo,
		txMgr,
	)

	// 5. Initialize Application Services
	authService := service.NewAuthService(&cfg.Auth, userRepo, sessionRepo, otpRepo, txMgr, nil)
	discoveryService := service.NewDiscoveryService(businessRepo, menuRepo, updateRepo, reviewRepo)
	customerService := service.NewCustomerService(customerListRepo, businessRepo, claimRepo, subRepo)
	businessService := service.NewBusinessService(businessRepo, ownershipRepo, authzService, txMgr)
	menuService := service.NewMenuService(menuRepo, authzService, txMgr)
	updatesService := service.NewUpdatesService(updateRepo, authzService)
	reservationService := service.NewReservationService(reservationRepo, businessRepo, authzService, txMgr)
	reviewService := service.NewReviewService(reviewRepo, reservationRepo, authzService, txMgr)
	reportService := service.NewReportService(reportRepo, reviewRepo, businessRepo)
	adminService := service.NewAdminService(
		ownershipWorkflow,
		authzService,
		subRepo,
		claimRepo,
		businessRepo,
		reviewRepo,
		reportRepo,
		adminActionRepo,
		txMgr,
	)

	// 6. Initialize HTTP Handlers
	handlers := &router.Handlers{
		Health:      handler.NewHealthHandler(pool),
		Auth:        handler.NewAuthHandler(authService, &cfg.Auth),
		Discovery:   handler.NewDiscoveryHandler(discoveryService),
		Customer:    handler.NewCustomerHandler(customerService),
		Business:    handler.NewBusinessHandler(businessService),
		Menu:        handler.NewMenuHandler(menuService),
		Updates:     handler.NewUpdatesHandler(updatesService),
		Reservation: handler.NewReservationHandler(reservationService),
		Review:      handler.NewReviewHandler(reviewService),
		Report:      handler.NewReportHandler(reportService),
		Admin:       handler.NewAdminHandler(adminService),
	}

	// 7. Wire HTTP Router with Middleware Pipeline
	httpHandler := router.NewRouter(cfg, authService, handlers)

	// 8. Configure HTTP Server with strict timeouts
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpHandler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	// 9. Start Server in background
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info(fmt.Sprintf("server listening on %s", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 10. Graceful Shutdown on SIGINT/SIGTERM
	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("server error encountered", "error", err)
		os.Exit(1)
	case sig := <-shutdownSignals:
		logger.Info("shutdown signal received", "signal", sig.String())

		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelShutdown()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful server shutdown failed", "error", err)
			_ = srv.Close()
		}

		logger.Info("closing database connection pool")
		pool.Close()
		logger.Info("server exited cleanly")
	}
}
