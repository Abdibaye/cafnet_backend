package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/service/assistant"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("connection error: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	docRepo := repository.NewPostgresDocumentRepository(pool)
	aiCfg := config.LoadAIConfig()
	ragClient := assistant.NewGeminiRAGClient(aiCfg)
	ingestionSvc := assistant.NewIngestionService(pool, docRepo, ragClient)

	fmt.Println("Indexing restaurant documents into pgvector...")
	total, updated, err := ingestionSvc.SyncAllBusinesses(ctx)
	if err != nil {
		fmt.Printf("sync error: %v\n", err)
		os.Exit(1)
	}

	count, _ := docRepo.Count(ctx)
	fmt.Printf("Ingestion complete! Total expected: %d, Updated/stored: %d, Current total in DB: %d\n", total, updated, count)
}
