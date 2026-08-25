package httpapi

import (
	"net/http"
)

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	if err := s.workflow.Health(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: map[string]string{"status": "ok"}})
}

func (s *Server) Workbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := webAssets.ReadFile("web/index.html")
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) ListCases(w http.ResponseWriter, r *http.Request) {
	values, err := s.workflow.ListCases(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: presentCaseList(values), Meta: map[string]int{"count": len(values)}})
}

func (s *Server) GetCase(w http.ResponseWriter, r *http.Request) {
	value, err := s.workflow.GetCase(r.Context(), r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: presentCase(value)})
}

func (s *Server) GetTimeline(w http.ResponseWriter, r *http.Request) {
	value, err := s.workflow.Timeline(r.Context(), r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: value, Meta: map[string]int{"count": len(value)}})
}

func (s *Server) GetApproval(w http.ResponseWriter, r *http.Request) {
	value, err := s.workflow.ApprovalEvidence(r.Context(), r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: presentApproval(value)})
}
