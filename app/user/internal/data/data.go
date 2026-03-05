package data

import (
	"os"
	"path/filepath"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/J-Y-Zhang/light-cloud-disk/app/user/internal/conf"
)

var ProviderSet = wire.NewSet(NewData, NewUserRepo)

type Data struct {
	db *gorm.DB
}

// openDB creates a *gorm.DB based on the configured driver ("mysql" or "sqlite").
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

	cleanup := func() {
		helper.Info("Closing user-service data resources.")
		if err := sqlDB.Close(); err != nil {
			helper.Errorf("Failed to close database connection cleanly: %v", err)
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
