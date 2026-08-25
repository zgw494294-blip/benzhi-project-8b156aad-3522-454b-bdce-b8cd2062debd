package httpapi

import (
	"io/fs"
	"net/http"
	"stone-restoration-trial/internal/workflow"
)

type Server struct {
	workflow *workflow.Service
	assets   fs.FS
}

func New(service *workflow.Service) *Server {
	assets, err := fs.Sub(webAssets, "web")
	if err != nil {
		panic(err)
	}
	return &Server{workflow: service, assets: assets}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.Health)
	mux.HandleFunc("GET /api/v1/restoration-cases", s.ListCases)
	mux.HandleFunc("POST /api/v1/restoration-cases", s.CreateCase)
	mux.HandleFunc("GET /api/v1/restoration-cases/{caseID}", s.GetCase)
	mux.HandleFunc("POST /api/v1/restoration-cases/{caseID}/formulas", s.AddFormula)
	mux.HandleFunc("POST /api/v1/restoration-cases/{caseID}/baseline-revisions", s.ReviseBaseline)
	mux.HandleFunc("POST /api/v1/restoration-cases/{caseID}/patches", s.AddPatch)
	mux.HandleFunc("POST /api/v1/restoration-cases/{caseID}/patches/{patchID}/observations", s.RecordObservation)
	mux.HandleFunc("POST /api/v1/restoration-cases/{caseID}/deviations/{deviationID}/remediation", s.RemediateDeviation)
	mux.HandleFunc("POST /api/v1/restoration-cases/{caseID}/submit-review", s.SubmitReview)
	mux.HandleFunc("POST /api/v1/restoration-cases/{caseID}/review-decisions", s.ReviewDecision)
	mux.HandleFunc("GET /api/v1/restoration-cases/{caseID}/timeline", s.GetTimeline)
	mux.HandleFunc("GET /api/v1/restoration-cases/{caseID}/approval", s.GetApproval)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", cacheAssets(http.FileServer(http.FS(s.assets)))))
	mux.HandleFunc("GET /", s.Workbench)
	return requestMiddleware(mux)
}

func requestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func cacheAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		next.ServeHTTP(w, r)
	})
}
