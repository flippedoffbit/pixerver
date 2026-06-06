package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"pixerver/handlers"
	"pixerver/internal/auth"
	"pixerver/internal/env"
	"pixerver/internal/uuidv7"
	"pixerver/logger"
	"pixerver/pipeline"
	"pixerver/queue"
	"pixerver/uploads"
)

func main() {
	// load .env early (if present) so other packages can rely on env vars
	if _, err := os.Stat(".env"); err == nil {
		if _, err := env.Load(".env"); err != nil {
			// don't fail startup for an empty or partially malformed .env; just warn
			logger.Warnf("failed to load .env: %v", err)
		} else {
			logger.Infof("loaded .env")
		}
	}

	// Initialize package logger with defaults; enable debug for demo
	cfg := logger.Config{}
	trueVal := true
	cfg.Debug.Enabled = &trueVal
	logger.Init(cfg)

	logger.Debug("debug message: starting application")
	logger.Info("info message: application running")
	logger.Warn("warn message: demo warning")
	logger.Error("error message: demo error")

	if v, err := auth.NewValidatorFromEnv("POSTFORM_JWT_SECRET", "POSTFORM_JWT_AUDIENCE", "POSTFORM_JWT_ISSUER"); err == nil {
		uploadDir := os.Getenv("POSTFORM_UPLOAD_DIR")
		if uploadDir == "" {
			uploadDir = "uploads"
		}
		outputDir := os.Getenv("PIXERVER_OUTPUT_DIR")
		if outputDir == "" {
			outputDir = uploadDir + "/processed"
		}
		tokenPath := os.Getenv("PIXERVER_TOKEN_PATH")
		if tokenPath == "" {
			if _, err := os.Stat("baseToken.json"); err == nil {
				tokenPath = "baseToken.json"
			}
		}
		srv := handlers.NewServer(v, uploadDir)
		srv.TokenPath = tokenPath
		srv.HTTPClient = &http.Client{Timeout: 30 * time.Second}
		resolver, resolverErr := pipeline.NewRedisBackendResolver(os.Getenv("PIXERVER_BACKEND_CONFIG_PREFIX"))
		if resolverErr != nil {
			logger.Warnf("backend config resolver: redis unavailable, env/direct configs only: %v", resolverErr)
		} else {
			logger.Infof("backend config resolver: redis enabled")
		}
		srv.Processor = pipeline.Processor{
			OutputDir:       outputDir,
			HTTPClient:      srv.HTTPClient,
			BackendResolver: resolver,
		}
		uploadRepo, uploadQueue, queueErr := setupUploadQueue()
		if queueErr != nil {
			logger.Warnf("upload queue unavailable: %v", queueErr)
		} else {
			signer, signerErr := uploads.NewCallbackSignerFromEnv()
			if signerErr != nil {
				logger.Warnf("callback jwt signer unavailable: %v", signerErr)
			}
			srv.Uploads = &uploads.Service{
				Repo:        uploadRepo,
				Queue:       uploadQueue,
				Processor:   srv.Processor,
				MaxAttempts: envInt("PIXERVER_JOB_MAX_ATTEMPTS", 3),
				Signer:      signer,
			}
			startWorkers(context.Background(), srv.Uploads, uploadQueue, envInt("PIXERVER_WORKER_COUNT", 1))
			logger.Infof("upload queue enabled")
		}
		http.HandleFunc("/upload", srv.PostFormHandler)
		http.HandleFunc("/uploads/", srv.UploadStatusHandler)
		logger.Infof("postform handler registered at /upload, uploads -> %s, processed -> %s", uploadDir, outputDir)
		if tokenPath != "" {
			logger.Infof("postform default token path -> %s", tokenPath)
		}
	} else {
		logger.Warnf("postform: validator not configured from env: %v", err)
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	addr := os.Getenv("PIXERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	logger.Infof("listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Errorf("server stopped: %v", err)
		os.Exit(1)
	}
}

func setupUploadQueue() (*uploads.RedisRepository, *queue.Queue, error) {
	repo, err := uploads.NewRedisRepository()
	if err != nil {
		return nil, nil, err
	}
	stream := os.Getenv("PIXERVER_QUEUE_STREAM")
	if stream == "" {
		stream = "pixerver:jobs"
	}
	group := os.Getenv("PIXERVER_QUEUE_GROUP")
	if group == "" {
		group = "pixerver-workers"
	}
	q, err := queue.New(stream, group, workerConsumer())
	if err != nil {
		_ = repo.Close()
		return nil, nil, err
	}
	return repo, q, nil
}

func startWorkers(ctx context.Context, service *uploads.Service, q *queue.Queue, count int) {
	if count < 1 {
		count = 1
	}
	for i := 0; i < count; i++ {
		worker := uploads.Worker{Service: service, Queue: q}
		go worker.Run(ctx)
	}
}

func workerConsumer() string {
	if v := os.Getenv("PIXERVER_WORKER_CONSUMER"); v != "" {
		return v
	}
	host, err := os.Hostname()
	if err == nil && host != "" {
		return host + "-" + strconv.Itoa(os.Getpid())
	}
	return uuidv7.New()
}

func envInt(name string, fallback int) int {
	if raw := os.Getenv(name); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			return v
		}
	}
	return fallback
}
