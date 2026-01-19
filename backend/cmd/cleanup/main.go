package main

import (
	"fmt"
	"log"
	"os"

	"github.com/irisdrone/backend/database"
	"github.com/irisdrone/backend/models"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Connect to database
	if err := database.Connect(); err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer database.Close()

	fmt.Println("Start cleanup...")

	// Delete all TrafficViolations
	if err := database.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.TrafficViolation{}).Error; err != nil {
		log.Fatalf("Failed to delete traffic violations: %v", err)
	}
	fmt.Println("✅ Deleted all traffic violations")

	// Delete all CrowdAlerts
	if err := database.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.CrowdAlert{}).Error; err != nil {
		log.Fatalf("Failed to delete crowd alerts: %v", err)
	}
	fmt.Println("✅ Deleted all crowd alerts")

	// Delete all Events
	if err := database.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Event{}).Error; err != nil {
		log.Fatalf("Failed to delete events: %v", err)
	}
	fmt.Println("✅ Deleted all events")

	// Delete all VehicleDetections
	if err := database.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.VehicleDetection{}).Error; err != nil {
		log.Fatalf("Failed to delete vehicle detections: %v", err)
	}
	fmt.Println("✅ Deleted all vehicle detections")

	// Delete all CrowdAnalyses
	if err := database.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.CrowdAnalysis{}).Error; err != nil {
		log.Fatalf("Failed to delete crowd analyses: %v", err)
	}
	fmt.Println("✅ Deleted all crowd analyses")

	// Delete devices of type CAMERA
	if err := database.DB.Where("type = ?", models.DeviceTypeCamera).Delete(&models.Device{}).Error; err != nil {
		log.Fatalf("Failed to delete camera devices: %v", err)
	}
	fmt.Println("✅ Deleted all camera devices")
    
    fmt.Println("Cleanup finished successfully")
}
