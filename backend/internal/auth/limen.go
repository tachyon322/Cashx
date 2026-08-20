package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/thecodearcher/limen"
	sqladapter "github.com/thecodearcher/limen/adapters/sql"
	credentialpassword "github.com/thecodearcher/limen/plugins/credential-password"

	"cashx/internal/platform"
)

// Limen wraps the Limen auth library plus the credential-password plugin API.
// It owns users (email+password, Argon2id), sessions and the session cookie;
// CashX domain rules (roles, approval, blocking, referral binding, wallets)
// stay in our tables and middleware.
type Limen struct {
	*limen.Limen
	Password credentialpassword.API
}

// uuidGenerator makes Limen generate uuid ids for users, sessions and
// verifications (matches the migrations; gen_random_uuid() covers any path
// where Limen does not fill the id itself).
type uuidGenerator struct{}

func (uuidGenerator) Generate(ctx context.Context) (any, error) { return uuid.NewString(), nil }
func (uuidGenerator) GetColumnType() limen.ColumnType           { return limen.ColumnTypeUUID }

// New builds the Limen instance over the existing application pool.
func New(cfg platform.Config, pool *pgxpool.Pool) (*Limen, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	adapter := sqladapter.NewPostgreSQL(sqlDB)

	// Password policy mirrors the legacy CashX contract (length >= 8 only);
	// the Limen defaults additionally require uppercase and digits, which
	// would reject existing dev credentials (admin1234).
	passwordPlugin := credentialpassword.New(
		credentialpassword.WithPasswordRequireUppercase(false),
		credentialpassword.WithPasswordRequireNumbers(false),
		credentialpassword.WithPasswordRequireSymbols(false),
	)

	config := &limen.Config{
		BaseURL: cfg.APIOrigin,
		Database: adapter,
		Secret:   []byte(cfg.SessionSecret),
		Session: limen.NewDefaultSessionConfig(
			limen.WithSessionShortDuration(0), // remember-me off: one 7-day TTL
		),
		Email: limen.NewDefaultEmailConfig(
			limen.WithEmailVerification(limen.WithDisableEmailVerification()),
		),
		Schema: limen.NewDefaultSchemaConfig(
			limen.WithSchemaIDGenerator(&uuidGenerator{}),
			limen.WithSchemaUser(limen.WithUserIncludeNameFields(false)),
		),
		HTTP: limen.NewDefaultHTTPConfig(
			limen.WithHTTPBasePath("/api/v1/auth"),
			limen.WithHTTPTrustedOrigins([]string{cfg.FrontendOrigin}),
			limen.WithHTTPOriginCheck(false), // curl/Go test clients send no Origin
			limen.WithHTTPCookieSecure(cfg.Env == "production"),
			limen.WithHTTPSessionCookieName("cashx_session"),
			limen.WithHTTPRateLimiter(limen.WithRateLimiterEnabled(false)), // our Redis limiter
			// These routes are served by our wrappers / not exposed:
			// me (own /auth/me), signup (registration only via /auth/register),
			// passwords-request-reset / passwords-reset (own dev-token wrappers).
			limen.WithHTTPDisabledPaths([]string{"me", "signup", "passwords-request-reset", "passwords-reset"}),
		),
		Plugins: []limen.Plugin{passwordPlugin},
	}

	auth, err := limen.New(config)
	if err != nil {
		return nil, err
	}
	return &Limen{Limen: auth, Password: credentialpassword.Use(auth)}, nil
}
