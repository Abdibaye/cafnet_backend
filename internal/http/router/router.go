package router

import (
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/http/handler"
	"github.com/Abdibaye/cafnet_backend/internal/http/middleware"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// Handlers encapsulates all HTTP handlers across product domains.
type Handlers struct {
	Health      *handler.HealthHandler
	Auth        *handler.AuthHandler
	Discovery   *handler.DiscoveryHandler
	Customer    *handler.CustomerHandler
	Business    *handler.BusinessHandler
	Menu        *handler.MenuHandler
	Updates     *handler.UpdatesHandler
	Reservation *handler.ReservationHandler
	Review      *handler.ReviewHandler
	Report      *handler.ReportHandler
	Admin       *handler.AdminHandler
}

// NewRouter constructs a versioned, secure HTTP handler tree using Go standard net/http.ServeMux.
func NewRouter(cfg *config.ServerConfig, authService service.AuthService, h *Handlers) http.Handler {
	mux := http.NewServeMux()

	authOptional := middleware.Authenticate(authService, cfg.Auth.SessionCookieName, false)
	authRequired := middleware.Authenticate(authService, cfg.Auth.SessionCookieName, true)

	// 1. Health & Readiness
	mux.HandleFunc("GET /health", h.Health.Liveness)
	mux.HandleFunc("GET /ready", h.Health.Readiness)
	mux.HandleFunc("GET /api/v1/health", h.Health.Liveness)
	mux.HandleFunc("GET /api/v1/ready", h.Health.Readiness)

	// 2. Authentication (/api/v1/auth)
	mux.HandleFunc("POST /api/v1/auth/otp/request", h.Auth.RequestOTP)
	mux.HandleFunc("POST /api/v1/auth/otp/verify", h.Auth.VerifyOTP)
	mux.HandleFunc("POST /api/v1/auth/logout", h.Auth.Logout)
	mux.Handle("GET /api/v1/auth/me", authRequired(http.HandlerFunc(h.Auth.Me)))

	// 3. Discovery & Public Profile (/api/v1/businesses, /api/v1/slugs)
	mux.Handle("GET /api/v1/businesses", authOptional(http.HandlerFunc(h.Discovery.ListBusinesses)))
	mux.Handle("GET /api/v1/businesses/{id}", authOptional(http.HandlerFunc(h.Discovery.GetBusinessProfile)))
	mux.Handle("GET /api/v1/slugs/{slug}", authOptional(http.HandlerFunc(h.Discovery.GetBusinessBySlug)))
	mux.Handle("GET /api/v1/search", authOptional(http.HandlerFunc(h.Discovery.Search)))

	// 4. Customer Interactions (/api/v1/saved, /api/v1/following, /api/v1/claims, /api/v1/submissions)
	mux.Handle("POST /api/v1/saved/{businessID}", authRequired(http.HandlerFunc(h.Customer.SaveBusiness)))
	mux.Handle("DELETE /api/v1/saved/{businessID}", authRequired(http.HandlerFunc(h.Customer.UnsaveBusiness)))
	mux.Handle("GET /api/v1/saved", authRequired(http.HandlerFunc(h.Customer.ListSavedBusinesses)))

	mux.Handle("POST /api/v1/following/{businessID}", authRequired(http.HandlerFunc(h.Customer.FollowBusiness)))
	mux.Handle("DELETE /api/v1/following/{businessID}", authRequired(http.HandlerFunc(h.Customer.UnfollowBusiness)))
	mux.Handle("GET /api/v1/following", authRequired(http.HandlerFunc(h.Customer.ListFollowedBusinesses)))

	mux.Handle("POST /api/v1/claims", authRequired(http.HandlerFunc(h.Customer.SubmitClaim)))
	mux.Handle("GET /api/v1/claims", authRequired(http.HandlerFunc(h.Customer.ListMyClaims)))
	mux.Handle("GET /api/v1/claims/{id}", authRequired(http.HandlerFunc(h.Customer.GetMyClaim)))

	mux.Handle("POST /api/v1/submissions", authRequired(http.HandlerFunc(h.Customer.CreateSubmission)))
	mux.Handle("GET /api/v1/submissions", authRequired(http.HandlerFunc(h.Customer.ListMySubmissions)))
	mux.Handle("GET /api/v1/submissions/{id}", authRequired(http.HandlerFunc(h.Customer.GetMySubmission)))

	// 5. Business Management (/api/v1/businesses/managed, /api/v1/businesses/{id}/manage, etc.)
	mux.Handle("GET /api/v1/businesses/managed", authRequired(http.HandlerFunc(h.Business.ListManagedBusinesses)))
	mux.Handle("GET /api/v1/businesses/{id}/manage", authRequired(http.HandlerFunc(h.Business.GetManagedBusiness)))
	mux.Handle("PATCH /api/v1/businesses/{id}", authRequired(http.HandlerFunc(h.Business.UpdateBusiness)))
	mux.Handle("GET /api/v1/businesses/{id}/hours", authRequired(http.HandlerFunc(h.Business.GetHours)))
	mux.Handle("PUT /api/v1/businesses/{id}/hours", authRequired(http.HandlerFunc(h.Business.SetHours)))
	mux.Handle("GET /api/v1/businesses/{id}/images", authRequired(http.HandlerFunc(h.Business.GetImages)))
	mux.Handle("PUT /api/v1/businesses/{id}/images", authRequired(http.HandlerFunc(h.Business.SetImages)))

	// 6. Menu Management
	mux.Handle("GET /api/v1/businesses/{id}/menu", authOptional(http.HandlerFunc(h.Menu.GetFullMenu)))
	mux.Handle("POST /api/v1/businesses/{id}/menu/categories", authRequired(http.HandlerFunc(h.Menu.CreateCategory)))
	mux.Handle("PATCH /api/v1/businesses/{id}/menu/categories/{categoryID}", authRequired(http.HandlerFunc(h.Menu.UpdateCategory)))
	mux.Handle("DELETE /api/v1/businesses/{id}/menu/categories/{categoryID}", authRequired(http.HandlerFunc(h.Menu.DeleteCategory)))
	mux.Handle("PUT /api/v1/businesses/{id}/menu/categories/reorder", authRequired(http.HandlerFunc(h.Menu.ReorderCategories)))
	mux.Handle("POST /api/v1/businesses/{id}/menu/items", authRequired(http.HandlerFunc(h.Menu.CreateItem)))
	mux.Handle("PATCH /api/v1/businesses/{id}/menu/items/{itemID}", authRequired(http.HandlerFunc(h.Menu.UpdateItem)))
	mux.Handle("DELETE /api/v1/businesses/{id}/menu/items/{itemID}", authRequired(http.HandlerFunc(h.Menu.DeleteItem)))
	mux.Handle("PUT /api/v1/businesses/{id}/menu/categories/{categoryID}/items/reorder", authRequired(http.HandlerFunc(h.Menu.ReorderItems)))

	// 7. Venue Updates
	mux.Handle("GET /api/v1/updates", authOptional(http.HandlerFunc(h.Updates.ListActiveUpdates)))
	mux.Handle("GET /api/v1/following/updates", authRequired(http.HandlerFunc(h.Updates.ListFollowingUpdates)))
	mux.Handle("GET /api/v1/businesses/{id}/updates", authOptional(http.HandlerFunc(h.Updates.ListUpdates)))
	mux.Handle("POST /api/v1/businesses/{id}/updates", authRequired(http.HandlerFunc(h.Updates.CreateUpdate)))
	mux.Handle("PATCH /api/v1/businesses/{id}/updates/{updateID}", authRequired(http.HandlerFunc(h.Updates.UpdateUpdate)))
	mux.Handle("DELETE /api/v1/businesses/{id}/updates/{updateID}", authRequired(http.HandlerFunc(h.Updates.DeleteUpdate)))

	// 8. Reservations
	mux.Handle("POST /api/v1/reservations", authRequired(http.HandlerFunc(h.Reservation.CreateReservation)))
	mux.Handle("GET /api/v1/reservations", authRequired(http.HandlerFunc(h.Reservation.ListCustomerReservations)))
	mux.Handle("GET /api/v1/reservations/{id}", authRequired(http.HandlerFunc(h.Reservation.GetCustomerReservation)))
	mux.Handle("POST /api/v1/reservations/{id}/cancel", authRequired(http.HandlerFunc(h.Reservation.CancelReservation)))

	mux.Handle("GET /api/v1/businesses/{id}/reservations", authRequired(http.HandlerFunc(h.Reservation.ListBusinessReservations)))
	mux.Handle("GET /api/v1/businesses/{id}/reservations/{resID}", authRequired(http.HandlerFunc(h.Reservation.GetBusinessReservation)))
	mux.Handle("POST /api/v1/businesses/{id}/reservations/{resID}/confirm", authRequired(http.HandlerFunc(h.Reservation.ConfirmReservation)))
	mux.Handle("POST /api/v1/businesses/{id}/reservations/{resID}/decline", authRequired(http.HandlerFunc(h.Reservation.DeclineReservation)))
	mux.Handle("POST /api/v1/businesses/{id}/reservations/{resID}/complete", authRequired(http.HandlerFunc(h.Reservation.CompleteReservation)))

	// 9. Reviews & Engagement
	mux.Handle("GET /api/v1/businesses/{id}/reviews", authOptional(http.HandlerFunc(h.Review.ListBusinessReviews)))
	mux.Handle("GET /api/v1/businesses/{id}/reviews/summary", authOptional(http.HandlerFunc(h.Review.GetReviewSummary)))
	mux.Handle("POST /api/v1/reviews", authRequired(http.HandlerFunc(h.Review.CreateReview)))
	mux.Handle("PATCH /api/v1/reviews/{id}", authRequired(http.HandlerFunc(h.Review.UpdateReview)))
	mux.Handle("DELETE /api/v1/reviews/{id}", authRequired(http.HandlerFunc(h.Review.DeleteReview)))
	mux.Handle("POST /api/v1/reviews/{id}/helpful", authRequired(http.HandlerFunc(h.Review.MarkHelpful)))
	mux.Handle("DELETE /api/v1/reviews/{id}/helpful", authRequired(http.HandlerFunc(h.Review.UnmarkHelpful)))
	mux.Handle("GET /api/v1/reviews/mine", authRequired(http.HandlerFunc(h.Review.ListMyReviews)))

	// 10. Community Reports
	mux.Handle("POST /api/v1/reviews/{id}/reports", authRequired(http.HandlerFunc(h.Report.ReportReview)))
	mux.Handle("POST /api/v1/businesses/{id}/reports", authRequired(http.HandlerFunc(h.Report.ReportBusiness)))

	// 11. Platform Administration (/api/v1/admin)
	mux.Handle("GET /api/v1/admin/stats", authRequired(http.HandlerFunc(h.Admin.GetStats)))

	mux.Handle("GET /api/v1/admin/submissions", authRequired(http.HandlerFunc(h.Admin.ListSubmissions)))
	mux.Handle("GET /api/v1/admin/submissions/{id}", authRequired(http.HandlerFunc(h.Admin.GetSubmission)))
	mux.Handle("POST /api/v1/admin/submissions/{id}/approve", authRequired(http.HandlerFunc(h.Admin.ApproveSubmission)))
	mux.Handle("POST /api/v1/admin/submissions/{id}/reject", authRequired(http.HandlerFunc(h.Admin.RejectSubmission)))

	mux.Handle("GET /api/v1/admin/claims", authRequired(http.HandlerFunc(h.Admin.ListClaims)))
	mux.Handle("GET /api/v1/admin/claims/{id}", authRequired(http.HandlerFunc(h.Admin.GetClaim)))
	mux.Handle("POST /api/v1/admin/claims/{id}/approve", authRequired(http.HandlerFunc(h.Admin.ApproveClaim)))
	mux.Handle("POST /api/v1/admin/claims/{id}/reject", authRequired(http.HandlerFunc(h.Admin.RejectClaim)))

	mux.Handle("GET /api/v1/admin/reviews/reports", authRequired(http.HandlerFunc(h.Admin.ListReviewReports)))
	mux.Handle("GET /api/v1/admin/reviews/reports/{id}", authRequired(http.HandlerFunc(h.Admin.GetReviewReport)))
	mux.Handle("POST /api/v1/admin/reviews/reports/{id}/resolve", authRequired(http.HandlerFunc(h.Admin.ResolveReviewReport)))
	mux.Handle("GET /api/v1/admin/reviews/{id}", authRequired(http.HandlerFunc(h.Admin.GetReview)))
	mux.Handle("POST /api/v1/admin/reviews/{id}/moderate", authRequired(http.HandlerFunc(h.Admin.ModerateReview)))

	mux.Handle("GET /api/v1/admin/business-reports", authRequired(http.HandlerFunc(h.Admin.ListBusinessReports)))
	mux.Handle("GET /api/v1/admin/business-reports/{id}", authRequired(http.HandlerFunc(h.Admin.GetBusinessReport)))
	mux.Handle("POST /api/v1/admin/business-reports/{id}/resolve", authRequired(http.HandlerFunc(h.Admin.ResolveBusinessReport)))

	mux.Handle("POST /api/v1/admin/businesses/{id}/publication", authRequired(http.HandlerFunc(h.Admin.SetBusinessPublicationStatus)))
	mux.Handle("POST /api/v1/admin/businesses/{id}/verification", authRequired(http.HandlerFunc(h.Admin.SetBusinessVerificationStatus)))
	mux.Handle("GET /api/v1/admin/actions", authRequired(http.HandlerFunc(h.Admin.ListAdminActions)))

	// Global Middleware Pipeline: RequestID -> SecurityHeaders -> Logging -> Recovery -> CORS -> CSRF
	var chain http.Handler = mux
	chain = middleware.CSRF(cfg.AllowedOrigins, cfg.Auth.SessionCookieName)(chain)
	chain = middleware.CORS(cfg.AllowedOrigins)(chain)
	chain = middleware.Recovery(chain)
	chain = middleware.Logging(chain)
	chain = middleware.SecurityHeaders(chain)
	chain = middleware.RequestID(chain)

	return chain
}
