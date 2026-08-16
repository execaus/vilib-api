package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vilib-api/config"
	"vilib-api/internal/eventhandler"
	"vilib-api/internal/handler"
	"vilib-api/internal/kafka"
	"vilib-api/internal/outbox"
	"vilib-api/internal/repository"
	"vilib-api/internal/s3"
	"vilib-api/internal/saga"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// shutdownTimeout — время на штатное завершение HTTP-сервера при получении сигнала остановки.
const shutdownTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		zap.L().Fatal("failed to run application", zap.Error(err))
	}
}

func run() error {
	ctx := context.Background()

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if validateErr := cfg.Validate(); validateErr != nil {
		return fmt.Errorf("invalid config: %w", validateErr)
	}

	gin.SetMode(string(cfg.Server.Mode))
	if cfg.Server.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	db, pool, err := repository.NewPostgresDB(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	executorProvider := repository.NewExecutorProvider(db)
	repo := repository.NewRepository(executorProvider)

	s3Client, err := s3.NewClient(cfg.S3)
	if err != nil {
		return fmt.Errorf("failed to create s3 client: %w", err)
	}

	localMailBox := make(chan string, 1)
	svc := service.NewService(cfg, localMailBox, s3Client, repo)
	sagaRunner := saga.NewSagaRunner(svc, executorProvider)

	producer := kafka.NewProducer(cfg.Kafka.Brokers)
	relay := outbox.NewRelay(
		sagaRunner, repo.Outbox, producer, cfg.Kafka.OutboxPollInterval, cfg.Kafka.OutboxBatchSize, logger,
	)

	relayCtx, stopRelay := context.WithCancel(context.Background())
	relayDone := runRelay(relayCtx, relay)

	consumer := kafka.NewConsumer(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup, cfg.Kafka.TopicProcessingEvents)
	eh := eventhandler.NewEventHandler(sagaRunner)
	consumerCtx, stopConsumer := context.WithCancel(context.Background())
	consumerDone := runConsumer(consumerCtx, consumer, eh)

	h := handler.NewHandler(sagaRunner)
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: h.GetRouter(),
	}

	go func() {
		zap.L().Info("starting server", zap.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Error("failed to start server", zap.Error(err))
		}
	}()

	waitForShutdownSignal()

	return shutdown(srv, stopConsumer, consumerDone, consumer, stopRelay, relayDone, producer)
}

// runRelay запускает релей outbox-очереди в фоновой горутине и возвращает канал, закрываемый
// после завершения relay.Run — по нему дожидаются остановки при graceful shutdown.
func runRelay(ctx context.Context, relay *outbox.Relay) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		if err := relay.Run(ctx); err != nil {
			zap.L().Error("outbox relay stopped with error", zap.Error(err))
		}
	}()

	return done
}

// runConsumer запускает консьюмер событий обработки видео (video.processing-events, §7.2
// эпика) в фоновой горутине и возвращает канал, закрываемый после завершения consumer.Run —
// по нему дожидаются остановки при graceful shutdown (Э1-Т26).
func runConsumer(ctx context.Context, consumer *kafka.ReaderConsumer, eh *eventhandler.EventHandler) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		if err := consumer.Run(ctx, eh.Handle); err != nil {
			zap.L().Error("kafka consumer stopped with error", zap.Error(err))
		}
	}()

	return done
}

// waitForShutdownSignal блокируется до получения SIGINT или SIGTERM.
func waitForShutdownSignal() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

// shutdown останавливает компоненты приложения в порядке: HTTP-сервер → консьюмер событий
// обработки видео (дожидается завершения текущего обработчика и коммита offset'а, Э1-Т26) →
// релей outbox → продюсер Kafka → читатель консьюмера (§7.1, §7.2 эпика — сначала перестаём
// принимать новую работу и публиковать, затем закрываем транспорт).
func shutdown(
	srv *http.Server,
	stopConsumer context.CancelFunc,
	consumerDone <-chan struct{},
	consumer *kafka.ReaderConsumer,
	stopRelay context.CancelFunc,
	relayDone <-chan struct{},
	producer *kafka.WriterProducer,
) error {
	zap.L().Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	stopConsumer()
	<-consumerDone

	stopRelay()
	<-relayDone

	if err := producer.Close(); err != nil {
		zap.L().Error("failed to close kafka producer", zap.Error(err))
	}

	if err := consumer.Close(); err != nil {
		zap.L().Error("failed to close kafka consumer", zap.Error(err))
	}

	zap.L().Info("server exited properly")
	return nil
}
