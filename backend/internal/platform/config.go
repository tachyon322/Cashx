package platform

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration, sourced from environment variables.
// Значения считаются из окружения (через .env в dev, godotenv.Load()). Все
// чувствительные параметры (секреты, доступы к БД, пароль админа) обязаны
// быть заданы явно — дефолтов у них нет, см. validate().
type Config struct {
	Env            string
	Port           int
	DatabaseURL    string
	AdminDatabaseURL string
	RedisAddr      string
	SessionTTL     time.Duration
	SessionCookie  string
	SessionSecret  string
	FrontendOrigin string
	APIOrigin      string
	RedirectBase   string
	// IntegrationKeySecret signs clickTokens for the redirect service.
	ClickTokenSecret string
	ClickTokenTTL    time.Duration
	SMTPFrom         string
	// WebOrigin is the frontend origin used to build invite/redirect URLs.
	WebOrigin string
	// RateLimit enables Redis-backed rate limiting (disabled in tests).
	RateLimit bool
	// IntegrationKeyEncryptionKey encrypts integration key secrets at rest.
	IntegrationKeyEncryptionKey string
	// AdminEmail/AdminPassword seed the initial superadmin when no users exist.
	AdminEmail    string
	AdminPassword string
	// SMTP settings for outbound email (optional in dev).
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// Load читает конфигурацию из окружения. Опциональный .env в рабочей
// директории подхватывается godotenv'ом; уже заданные переменные окружения
// имеют приоритет и не перезаписываются. Чувствительные параметры обязаны
// быть заданы — при отсутствии возвращается ошибка со списком недостающих.
func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		// Не чувствительные операционные умолчания.
		Env:              getenv("CASHX_ENV", "development"),
		Port:             getenvInt("CASHX_PORT", 8080),
		RedisAddr:        getenv("CASHX_REDIS_ADDR", "localhost:6381"),
		SessionTTL:       getenvDuration("CASHX_SESSION_TTL", 30*24*time.Hour),
		SessionCookie:    getenv("CASHX_SESSION_COOKIE", "cashx_session"),
		FrontendOrigin:   getenv("CASHX_FRONTEND_ORIGIN", "http://localhost:3000"),
		APIOrigin:        getenv("CASHX_API_ORIGIN", "http://localhost:8080"),
		RedirectBase:     getenv("CASHX_REDIRECT_BASE", "http://localhost:8081"),
		ClickTokenTTL:    getenvDuration("CASHX_CLICK_TOKEN_TTL", 90*24*time.Hour),
		SMTPFrom:         getenv("CASHX_SMTP_FROM", "no-reply@cashx.local"),
		WebOrigin:        getenv("CASHX_WEB_ORIGIN", "http://localhost:3000"),
		RateLimit:        getenv("CASHX_RATE_LIMIT", "true") == "true",
		AdminEmail:       getenv("CASHX_ADMIN_EMAIL", "admin@cashx.local"),
		SMTPHost:         getenv("CASHX_SMTP_HOST", ""),
		SMTPPort:         getenvInt("CASHX_SMTP_PORT", 587),
		SMTPUsername:     getenv("CASHX_SMTP_USERNAME", ""),
		SMTPPassword:     getenv("CASHX_SMTP_PASSWORD", ""),

		// Чувствительные параметры: только из окружения, без дефолтов.
		DatabaseURL:                getenv("CASHX_DATABASE_URL", ""),
		AdminDatabaseURL:           getenv("CASHX_ADMIN_DATABASE_URL", ""),
		SessionSecret:              getenv("CASHX_SESSION_SECRET", ""),
		ClickTokenSecret:           getenv("CASHX_CLICK_TOKEN_SECRET", ""),
		IntegrationKeyEncryptionKey: getenv("CASHX_INTEGRATION_KEY_ENCRYPTION_KEY", ""),
		AdminPassword:              getenv("CASHX_ADMIN_PASSWORD", ""),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// validate проверяет, что все чувствительные параметры заданы.
func (c Config) validate() error {
	required := map[string]string{
		"CASHX_DATABASE_URL":                  c.DatabaseURL,
		"CASHX_ADMIN_DATABASE_URL":            c.AdminDatabaseURL,
		"CASHX_SESSION_SECRET":                c.SessionSecret,
		"CASHX_CLICK_TOKEN_SECRET":            c.ClickTokenSecret,
		"CASHX_INTEGRATION_KEY_ENCRYPTION_KEY": c.IntegrationKeyEncryptionKey,
		"CASHX_ADMIN_PASSWORD":                c.AdminPassword,
	}
	var missing []string
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf(
			"missing required environment variables: %s (задайте их или скопируйте backend/.env.example в backend/.env)",
			strings.Join(missing, ", "),
		)
	}
	return nil
}