package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"cashx/internal/api/gen"
	"cashx/internal/integrations"
	"cashx/internal/platform"
	"cashx/internal/platform/httpjson"
	"cashx/internal/platform/httpmw"
)

// Router builds the full API router with per-group authz middleware.
func (s *Server) Router(rdb *redis.Client) http.Handler {
	r := httpmw.Chain(s.Log, s.Cfg.FrontendOrigin)
	mw := s.Middleware

	noop := func(h http.Handler) http.Handler { return h }
	rl := func(scope string, limit int, key func(*http.Request) string) func(http.Handler) http.Handler {
		if !s.Cfg.RateLimit || rdb == nil {
			return noop
		}
		return platform.RateLimit(rdb, scope, limit, key)
	}

	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		httpjson.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	w := &gen.ServerInterfaceWrapper{Handler: s}
	r.Route("/api/v1", func(r chi.Router) {
		// Auth (no session required).
		r.Group(func(r chi.Router) {
			r.Use(rl("auth", 10, platform.RemoteIPKey))
			r.Post("/auth/register", w.AuthRegister)
			r.Post("/auth/password-reset/request", w.AuthPasswordResetRequest)
			r.Post("/auth/password-reset/confirm", w.AuthPasswordResetConfirm)
		})

		// Auth: current user (session required) — /auth/me не рейт-лимитим:
		// read-only проверка сессии на каждой загрузке SPA.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireUser)
			r.Get("/auth/me", w.AuthMe)
		})

		// Limen native auth routes (signin/credential, signout, sessions,
		// revoke-sessions). Mounted after our exact /api/v1/auth routes so
		// ours take precedence.
		r.Group(func(r chi.Router) {
			r.Use(rl("auth", 10, platform.RemoteIPKey))
			r.Mount("/auth", s.Limen.Handler())
		})

		// Partner cabinet.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireUser, mw.RequirePartner)
			r.Get("/cabinet/summary", w.CabinetSummary)
			r.Get("/cabinet/offers", w.CabinetOffersList)
			r.Post("/cabinet/offers/{offerId}/join", w.CabinetOfferJoin)
			r.Get("/cabinet/offers/{offerId}", w.CabinetOfferStats)
			r.Get("/cabinet/offers/{offerId}/history.csv", s.CabinetOfferHistoryCSV)
			r.Get("/cabinet/offers/{offerId}/sources", w.CabinetOfferSourcesList)
			r.Post("/cabinet/offers/{offerId}/sources", w.CabinetOfferSourceCreate)
			r.Patch("/cabinet/offers/{offerId}/sources/{sourceId}", w.CabinetOfferSourceUpdate)
			r.Delete("/cabinet/offers/{offerId}/sources/{sourceId}", w.CabinetOfferSourceDelete)
			r.Get("/cabinet/source-groups", w.CabinetSourceGroupsList)
			r.Post("/cabinet/source-groups", w.CabinetSourceGroupCreate)
			r.Patch("/cabinet/source-groups/{groupId}", w.CabinetSourceGroupUpdate)
			r.Delete("/cabinet/source-groups/{groupId}", w.CabinetSourceGroupDelete)
			r.Get("/cabinet/payouts/config", w.CabinetPayoutsConfig)
			r.Get("/cabinet/payouts", w.CabinetPayoutsList)
			r.Post("/cabinet/payouts/requests", w.CabinetPayoutRequest)
			r.Post("/cabinet/payouts/requests/{id}/cancel", w.CabinetPayoutCancel)
			r.Get("/cabinet/referrals", w.CabinetReferrals)
			r.Get("/cabinet/notifications", w.CabinetNotificationsList)
			r.Post("/cabinet/notifications/read-all", w.CabinetNotificationsReadAll)
			r.Post("/cabinet/notifications/{type}/{id}/read", w.CabinetNotificationRead)
			r.Put("/cabinet/profile", w.CabinetProfileUpdate)
		})

		// Admin: partners / projects / offers / integration keys.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireUser, mw.RequireStaff("project_manager"))
			r.Get("/admin/partners", w.AdminPartnersList)
			r.Get("/admin/partners/{id}", w.AdminPartnerGet)
			r.Post("/admin/partners", w.AdminPartnerCreate)
			r.Patch("/admin/partners/{id}", w.AdminPartnerUpdate)
			r.Post("/admin/partners/{id}/rate", w.AdminPartnerRate)
			r.Get("/admin/projects", w.AdminProjectsList)
			r.Post("/admin/projects", w.AdminProjectCreate)
			r.Patch("/admin/projects/{id}", w.AdminProjectUpdate)
			r.Get("/admin/offers", w.AdminOffersList)
			r.Post("/admin/offers", w.AdminOfferCreate)
			r.Patch("/admin/offers/{id}", w.AdminOfferUpdate)
			r.Post("/admin/offers/{id}/terms", w.AdminOfferTerms)
			r.Get("/admin/integration-keys", w.AdminIntegrationKeysList)
			r.Post("/admin/integration-keys", w.AdminIntegrationKeyCreate)
			r.Post("/admin/integration-keys/{keyId}/rotate", w.AdminIntegrationKeyRotate)
			r.Post("/admin/integration-keys/{keyId}/deactivate", w.AdminIntegrationKeyDeactivate)
		})

		// Admin: support read access to partners/offers/projects.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireUser, mw.RequireStaff("support"))
			r.Get("/admin/partners", w.AdminPartnersList)
			r.Get("/admin/partners/{id}", w.AdminPartnerGet)
			r.Get("/admin/projects", w.AdminProjectsList)
			r.Get("/admin/offers", w.AdminOffersList)
		})

		// Admin: finance.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireUser, mw.RequireStaff("finance"))
			r.Get("/admin/withdrawals", w.AdminWithdrawalsList)
			r.Post("/admin/withdrawals/{id}/decide", w.AdminWithdrawalDecide)
			r.Post("/admin/withdrawals/{id}/pay", w.AdminWithdrawalPay)
			r.Get("/admin/finance/rules", w.AdminFinanceRulesGet)
			r.Put("/admin/finance/rules", w.AdminFinanceRulesPut)
			r.Get("/admin/finance/ledger", w.AdminFinanceLedger)
			r.Get("/admin/finance/earnings", w.AdminFinanceEarnings)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireUser, mw.RequireStaff("finance"))
			r.Post("/admin/withdrawals/mirror", s.AdminWithdrawalMirror)
			r.Post("/admin/tracking_links/mirror", s.AdminTrackingLinkMirror)
		})

		// Admin: support read of withdrawals.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireUser, mw.RequireStaff("support"))
			r.Get("/admin/withdrawals", w.AdminWithdrawalsList)
		})

		// Admin: content.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireUser, mw.RequireStaff("content_manager"))
			r.Get("/admin/announcements", w.AdminAnnouncementsList)
			r.Post("/admin/announcements", w.AdminAnnouncementCreate)
			r.Patch("/admin/announcements/{id}", w.AdminAnnouncementUpdate)
			r.Delete("/admin/announcements/{id}", w.AdminAnnouncementDelete)
			r.Get("/admin/platform/branding", w.AdminBrandingGet)
			r.Put("/admin/platform/branding", w.AdminBrandingPut)
		})

		// Admin: staff management (superadmin only).
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireUser, mw.RequireStaff("superadmin"))
			r.Get("/admin/staff", s.AdminStaffList)
			r.Post("/admin/staff", s.AdminStaffCreate)
			r.Get("/admin/staff/{id}", s.AdminStaffGet)
			r.Patch("/admin/staff/{id}", s.AdminStaffUpdate)
		})

		// Admin: audit (all staff roles can read the trail).
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireUser, mw.RequireStaff("project_manager", "finance", "content_manager", "support"))
			r.Get("/admin/audit", w.AdminAuditList)
		})

		// Signed project events.
		r.Group(func(r chi.Router) {
			imw := &integrations.Middleware{Q: s.Q, Offers: s.Offers}
			r.Use(rl("integrations", 300, integrations.RateLimitKey), imw.Verify)
			r.Post("/integrations/events", w.IntegrationsEvent)
		})
	})

	return r
}
