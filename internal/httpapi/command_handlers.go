package httpapi

import (
	"context"
	"net/http"
	"stone-restoration-trial/internal/workflow"
)

func (s *Server) CreateCase(w http.ResponseWriter, r *http.Request) {
	var command workflow.CreateCaseCommand
	if err := decodeJSON(r, &command); err != nil {
		writeError(w, err)
		return
	}
	value, replayed, err := s.workflow.CreateCase(context.WithoutCancel(r.Context()), command)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, envelope{Data: value, Meta: replayMeta(replayed)})
}

func (s *Server) AddFormula(w http.ResponseWriter, r *http.Request) {
	var command workflow.AddFormulaCommand
	if err := decodeJSON(r, &command); err != nil {
		writeError(w, err)
		return
	}
	value, replayed, err := s.workflow.AddFormula(context.WithoutCancel(r.Context()), r.PathValue("caseID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: value, Meta: replayMeta(replayed)})
}

func (s *Server) ReviseBaseline(w http.ResponseWriter, r *http.Request) {
	var command workflow.ReviseBaselineCommand
	if err := decodeJSON(r, &command); err != nil {
		writeError(w, err)
		return
	}
	value, replayed, err := s.workflow.ReviseBaseline(context.WithoutCancel(r.Context()), r.PathValue("caseID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: value, Meta: replayMeta(replayed)})
}

func (s *Server) AddPatch(w http.ResponseWriter, r *http.Request) {
	var command workflow.AddPatchCommand
	if err := decodeJSON(r, &command); err != nil {
		writeError(w, err)
		return
	}
	value, replayed, err := s.workflow.AddPatch(context.WithoutCancel(r.Context()), r.PathValue("caseID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: value, Meta: replayMeta(replayed)})
}

func (s *Server) RecordObservation(w http.ResponseWriter, r *http.Request) {
	var command workflow.RecordObservationCommand
	if err := decodeJSON(r, &command); err != nil {
		writeError(w, err)
		return
	}
	value, replayed, err := s.workflow.RecordObservation(context.WithoutCancel(r.Context()), r.PathValue("caseID"), r.PathValue("patchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: value, Meta: replayMeta(replayed)})
}

func (s *Server) RemediateDeviation(w http.ResponseWriter, r *http.Request) {
	var command workflow.RemediateCommand
	if err := decodeJSON(r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.DeviationID = r.PathValue("deviationID")
	value, replayed, err := s.workflow.Remediate(context.WithoutCancel(r.Context()), r.PathValue("caseID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: value, Meta: replayMeta(replayed)})
}

func (s *Server) SubmitReview(w http.ResponseWriter, r *http.Request) {
	var command workflow.SubmitReviewCommand
	if err := decodeJSON(r, &command); err != nil {
		writeError(w, err)
		return
	}
	value, replayed, err := s.workflow.SubmitReview(context.WithoutCancel(r.Context()), r.PathValue("caseID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: value, Meta: replayMeta(replayed)})
}

func (s *Server) ReviewDecision(w http.ResponseWriter, r *http.Request) {
	var command workflow.ReviewDecisionCommand
	if err := decodeJSON(r, &command); err != nil {
		writeError(w, err)
		return
	}
	value, replayed, err := s.workflow.DecideReview(context.WithoutCancel(r.Context()), r.PathValue("caseID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: value, Meta: replayMeta(replayed)})
}
