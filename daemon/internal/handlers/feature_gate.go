// Package handlers provides HTTP request handlers.
// feature_gate.go: Middleware to gate API endpoints on feature flags.
package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"dplaned/internal/features"
)

// FeatureGateMiddleware returns HTTP 412 (Precondition Failed) if a feature is disabled.
// Registers route-to-feature mappings for automatic gating.
func FeatureGateMiddleware(featureManager *features.Manager) func(http.Handler) http.Handler {
	// Map routes to required features
	routeFeatures := map[string]string{
		"/api/ha/":              "ha_clustering",
		"/api/cluster/":         "ha_clustering",
		"/api/witness/":         "network_witness",
		"/api/nvmeof/":          "nvmeof_support",
		"/api/nfs/kerberos":     "kerberos_integration",
		"/api/compliance/":      "compliance_engine",
		"/api/system/license/":  "enterprise_licensing",
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this route requires a feature
			for routePrefix, requiredFeature := range routeFeatures {
				if strings.HasPrefix(r.URL.Path, routePrefix) {
					if !featureManager.IsEnabled(requiredFeature) {
						f, _ := featureManager.Get(requiredFeature)
						http.Error(w,
							fmt.Sprintf("Feature %s is not enabled (current state: %s)", requiredFeature, f.State),
							http.StatusPreconditionFailed) // 412
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// FeatureFlagsHandler returns GET /api/system/features with all feature states.
func FeatureFlagsHandler(featureManager *features.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		features := featureManager.List()

		// Return as JSON array
		w.Write([]byte("["))
		for i, f := range features {
			if i > 0 {
				w.Write([]byte(","))
			}
			fmt.Fprintf(w, `{"id":"%s","name":"%s","state":"%s"}`, f.ID, f.Name, f.State)
		}
		w.Write([]byte("]"))
	}
}

// FeatureEnableHandler handles POST /api/system/features/:id/enable
func FeatureEnableHandler(featureManager *features.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		featureID := strings.TrimPrefix(r.URL.Path, "/api/system/features/")
		featureID = strings.TrimSuffix(featureID, "/enable")

		state := r.URL.Query().Get("state")
		if state != "beta" && state != "stable" {
			state = "beta"
		}

		if err := featureManager.Enable(r.Context(), featureID, state); err != nil {
			http.Error(w, fmt.Sprintf("Failed to enable feature: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"enabled","feature":"%s","state":"%s"}`, featureID, state)
	}
}

// FeatureDisableHandler handles POST /api/system/features/:id/disable
func FeatureDisableHandler(featureManager *features.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		featureID := strings.TrimPrefix(r.URL.Path, "/api/system/features/")
		featureID = strings.TrimSuffix(featureID, "/disable")

		if err := featureManager.Disable(r.Context(), featureID); err != nil {
			http.Error(w, fmt.Sprintf("Failed to disable feature: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"disabled","feature":"%s"}`, featureID)
	}
}
