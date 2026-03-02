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
		helper.Errorf("failed to connect database: %v", err)
		return nil, nil, err
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(int(c.Database.MaxIdleConns))
	sqlDB.SetMaxOpenConns(int(c.Database.MaxOpenConns))

	cleanup := func() {
		helper.Info("closing data resources")
		if err := sqlDB.Close(); err != nil {
			helper.Errorf("failed to close database: %v", err)
		}
	}

	// Auto migrate
	if err := db.AutoMigrate(&UserPO{}); err != nil {
		helper.Errorf("auto migrate user table failed: %v", err)
	}

	return &Data{db: db}, cleanup, nil
}

// UserPO is defined in user.go but referenced here for migration.
// We keep it in user.go to keep domain-specific persistence together.
func HealthCheck(d *Data) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
