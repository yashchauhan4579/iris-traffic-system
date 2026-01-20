package main

import (
	"log"
	"os"

	"github.com/irisdrone/backend/database"
	"github.com/irisdrone/backend/models"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Connect to database
	database.Connect()

	log.Println("Cleaning up devices...")

	// Delete all devices
	if err := database.DB.Exec("DELETE FROM devices").Error; err != nil {
		log.Fatalf("Failed to delete devices: %v", err)
	}
    
    // Also clear assignments
    if err := database.DB.Exec("DELETE FROM worker_camera_assignments").Error; err != nil {
        log.Fatalf("Failed to delete assignments: %v", err)
    }

	log.Println("Successfully deleted all devices and assignments.")
}
