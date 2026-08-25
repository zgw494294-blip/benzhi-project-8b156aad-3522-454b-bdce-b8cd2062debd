package domain

import (
	"fmt"
	"time"
)

type DeviationSeed struct {
	Metric        string
	MeasuredValue string
	Threshold     string
	Severity      string
}

func EvaluateObservation(observation Observation, thresholds Thresholds) []DeviationSeed {
	result := make([]DeviationSeed, 0, 4)
	if observation.ColorDifference > thresholds.MaxColorDifference {
		result = append(result, numericDeviation("color_difference", observation.ColorDifference, thresholds.MaxColorDifference, true))
	}
	if observation.WaterAbsorption > thresholds.MaxWaterAbsorption {
		result = append(result, numericDeviation("water_absorption", observation.WaterAbsorption, thresholds.MaxWaterAbsorption, true))
	}
	if observation.AdhesionStrength < thresholds.MinAdhesionStrength {
		result = append(result, numericDeviation("adhesion_strength", observation.AdhesionStrength, thresholds.MinAdhesionStrength, false))
	}
	if len(observation.SurfaceDefects) > 0 {
		result = append(result, DeviationSeed{Metric: "surface_defect", MeasuredValue: fmt.Sprintf("%d 项", len(observation.SurfaceDefects)), Threshold: "无可见缺陷", Severity: "major"})
	}
	return result
}

func numericDeviation(metric string, measured, threshold float64, upper bool) DeviationSeed {
	distance := measured - threshold
	if !upper {
		distance = threshold - measured
	}
	ratio := distance / threshold
	severity := "minor"
	if ratio > 0.25 {
		severity = "major"
	}
	return DeviationSeed{Metric: metric, MeasuredValue: fmt.Sprintf("%.3f", measured), Threshold: fmt.Sprintf("%.3f", threshold), Severity: severity}
}

func ApplyFinalEvaluation(patch *TrialPatch, observation Observation, deviations []DeviationSeed, now time.Time) {
	conclusion := "passed"
	if len(deviations) > 0 {
		conclusion = "failed"
	}
	patch.EvaluationResult = &EvaluationResult{Conclusion: conclusion, EvaluatedAt: now.UTC(), DeviationCount: len(deviations)}
	completed := now.UTC()
	patch.CompletedAt = &completed
}
