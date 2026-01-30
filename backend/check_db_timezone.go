package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"github.com/joho/godotenv"
)

func main() {
    // Load .env explicitly
    if err := godotenv.Load(".env"); err != nil {
        log.Println("Warning: .env not found")
    }

	dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        log.Fatal("DATABASE_URL is empty")
    }

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	var now time.Time
	db.Raw("SELECT NOW()").Scan(&now)
	fmt.Printf("DB Time: %v\n", now)
    
    var tz string
    db.Raw("SHOW timezone").Scan(&tz)
    fmt.Printf("DB Configured Timezone: %s\n", tz)
}
