package marketplace

import "net/http"

var allowedOrigins = map[string]struct{}{
	"http://localhost:5174": {},
	"http://localhost:4180": {},
}

// NewRouter builds the marketplace HTTP handler with the complete /api surface.
func NewRouter(store *Store) http.Handler {
	handler := &Handler{store: store}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/healthz", handler.healthz)
	mux.HandleFunc("GET /api/me", handler.me)
	mux.HandleFunc("GET /api/tasks", handler.listTasks)
	mux.HandleFunc("POST /api/tasks", handler.publishTask)
	mux.HandleFunc("GET /api/tasks/{id}", handler.getTask)
	mux.HandleFunc("POST /api/tasks/{id}/accept", handler.acceptTask)
	mux.HandleFunc("GET /api/tasks/{id}/trace", handler.getTrace)
	mux.HandleFunc("GET /api/tasks/{id}/trust", handler.taskTrust)
	mux.HandleFunc("POST /api/tasks/{id}/contracts/sign", handler.signContract)
	mux.HandleFunc("GET /api/expert/overview", handler.expertOverview)
	mux.HandleFunc("GET /api/wallet", handler.wallet)
	mux.HandleFunc("POST /api/wallet/recharge", handler.rechargeWallet)
	mux.HandleFunc("POST /api/wallet/vip", handler.subscribeVip)
	mux.HandleFunc("GET /api/gateway", handler.gatewayOverview)
	mux.HandleFunc("POST /api/gateway/keys", handler.createApiKey)
	mux.HandleFunc("POST /api/gateway/keys/{id}/revoke", handler.revokeApiKey)
	mux.HandleFunc("GET /api/reputation/{userId}", handler.reputation)
	mux.HandleFunc("POST /api/ratings", handler.submitRating)
	mux.HandleFunc("POST /api/auth/register/{channel}", handler.registerChannel)
	mux.HandleFunc("/", handler.notFound)

	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowedOrigins[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			if _, ok := allowedOrigins[origin]; ok {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
