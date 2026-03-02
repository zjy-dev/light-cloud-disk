package data

import (
	"context"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/google/wire"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/conf"
)

var ProviderSet = wire.NewSet(NewData, NewFileRepo)

type Data struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	helper := log.NewHelper(logger)

	godotenv.Load()

	dsn := os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") +
		"@tcp(" + os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT") + ")/" +
		os.Getenv("DB_NAME") + "?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		helper.Errorf("failed to connect database: %v", err)
		return nil, nil, err
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(int(c.Database.MaxIdleConns))
	sqlDB.SetMaxOpenConns(int(c.Database.MaxOpenConns))

	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		helper.Errorf("failed to connect redis: %v", err)
		return nil, nil, err
	}

	// Auto migrate file-related tables
	if err := db.AutoMigrate(&FilePO{}, &FileStorePO{}, &SharePO{}); err != nil {
		helper.Errorf("auto migrate failed: %v", err)
	}

	cleanup := func() {
		helper.Info("closing data resources")
		sqlDB.Close()
		rdb.Close()
	}

	return &Data{db: db, redis: rdb}, cleanup, nil
}
