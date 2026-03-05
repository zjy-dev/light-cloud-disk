package data

import (
	"context"
	"os"
	"path/filepath"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/google/wire"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
)

var ProviderSet = wire.NewSet(NewData, NewFileRepo, NewMessageProducer, NewSeaweedFSClient, NewOSSClient)

type Data struct {
	db    *gorm.DB
	redis *redis.Client
}

// openDB creates a *gorm.DB based on the configured driver ("mysql" or "sqlite").
// Driver selection priority: DB_DRIVER env > config YAML driver field > "mysql" default.
func openDB(c *conf.Data, helper *log.Helper) (*gorm.DB, error) {
	driver := "mysql"
	if c.Database != nil && c.Database.Driver != "" {
		driver = c.Database.Driver
	}
	if env := os.Getenv("DB_DRIVER"); env != "" {
		driver = env
	}

	switch driver {
	case "sqlite":
		dbPath := os.Getenv("SQLITE_PATH")
		if dbPath == "" {
			dbPath = "./data.db"
		}
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		helper.Infof("Opening SQLite database at %s", dbPath)
		return gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	default:
		dsn := os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") +
			"@tcp(" + os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT") + ")/" +
			os.Getenv("DB_NAME") + "?charset=utf8mb4&parseTime=True&loc=Local"
		helper.Infof("Opening MySQL database at %s:%s", os.Getenv("DB_HOST"), os.Getenv("DB_PORT"))
		return gorm.Open(mysql.Open(dsn), &gorm.Config{})
	}
}

func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	helper := log.NewHelper(logger)

	godotenv.Load()

	db, err := openDB(c, helper)
	if err != nil {
		helper.Errorf("Failed to open database: %v", err)
		return nil, nil, err
	}

	sqlDB, _ := db.DB()
	if c.Database != nil {
		sqlDB.SetMaxIdleConns(int(c.Database.MaxIdleConns))
		sqlDB.SetMaxOpenConns(int(c.Database.MaxOpenConns))
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		helper.Errorf("Failed to connect to Redis: %v", err)
		return nil, nil, err
	}

	// Auto-migrate file-related tables
	if err := db.AutoMigrate(&FilePO{}, &FileStorePO{}, &SharePO{}); err != nil {
		helper.Errorf("Auto migration for file tables failed: %v", err)
	}

	cleanup := func() {
		helper.Info("Closing file-service data resources.")
		sqlDB.Close()
		rdb.Close()
	}

	return &Data{db: db, redis: rdb}, cleanup, nil
}
