package data

import (
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/J-Y-Zhang/light-cloud-disk/app/user/internal/conf"
)

var ProviderSet = wire.NewSet(NewData, NewUserRepo)

type Data struct {
	db *gorm.DB
}

func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	helper := log.NewHelper(logger)

	godotenv.Load()

	dsn := os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") +
		"@tcp(" + os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT") + ")/" +
		os.Getenv("DB_NAME") + "?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		helper.Errorf("Failed to connect to MySQL: %v", err)
		return nil, nil, err
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(int(c.Database.MaxIdleConns))
	sqlDB.SetMaxOpenConns(int(c.Database.MaxOpenConns))

	cleanup := func() {
		helper.Info("Closing user-service data resources.")
		if err := sqlDB.Close(); err != nil {
			helper.Errorf("Failed to close MySQL connection cleanly: %v", err)
		}
	}

	// Auto-migrate user table
	if err := db.AutoMigrate(&UserPO{}); err != nil {
		helper.Errorf("Auto migration for user table failed: %v", err)
	}

	return &Data{db: db}, cleanup, nil
}

// UserPO is defined in user.go and referenced here for migration
// Keep UserPO near domain persistence definitions in user.go
func HealthCheck(d *Data) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
