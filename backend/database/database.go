package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/irisdrone/backend/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Connect initializes the database connection
func Connect() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	var err error
	DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	
	// Increase max number of open connections from default (0=unlimited) to a safe limit, or as requested 'increase'.
	// Since user asked to increase, we set a high robust limit.
	sqlDB.SetMaxOpenConns(100) 
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("✅ Database connected successfully")

	// Auto-migrate models
	if err := autoMigrate(); err != nil {
		return fmt.Errorf("failed to auto-migrate: %w", err)
	}

	return nil
}

// autoMigrate runs database migrations
func autoMigrate() error {
	return DB.AutoMigrate(
		&models.Device{},
		&models.Event{},
		&models.Worker{},
		&models.WorkerToken{},
		&models.WorkerCameraAssignment{},
		&models.WorkerApprovalRequest{},
		&models.CrowdAnalysis{},
		&models.CrowdAlert{},
		&models.TrafficViolation{},
		&models.Vehicle{},
		&models.VehicleDetection{},
		&models.Watchlist{},
		&models.User{},
	)
}

// Close closes the database connection
func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

