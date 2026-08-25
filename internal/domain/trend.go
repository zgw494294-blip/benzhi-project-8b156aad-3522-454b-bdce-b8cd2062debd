package domain

func BuildStageTrends(patch TrialPatch, thresholds Thresholds) []StageTrend {
	result := make([]StageTrend, 0, len(patch.Observations))
	previousWorsening := map[string]bool{}
	for index, observation := range patch.Observations {
		stage := StageTrend{Stage: observation.Stage, ObservedAt: observation.ObservedAt, Metrics: make([]MetricTrend, 0, 3), Warnings: []TrendWarning{}}
		values := []struct {
			metric  string
			current float64
			limit   float64
			upper   bool
		}{
			{"color_difference", observation.ColorDifference, thresholds.MaxColorDifference, true},
			{"water_absorption", observation.WaterAbsorption, thresholds.MaxWaterAbsorption, true},
			{"adhesion_strength", observation.AdhesionStrength, thresholds.MinAdhesionStrength, false},
		}
		for _, value := range values {
			change := 0.0
			worsening := false
			if index > 0 {
				previous := patch.Observations[index-1]
				switch value.metric {
				case "color_difference":
					change = value.current - previous.ColorDifference
				case "water_absorption":
					change = value.current - previous.WaterAbsorption
				case "adhesion_strength":
					change = value.current - previous.AdhesionStrength
				}
				worsening = change > 0
				if !value.upper {
					worsening = change < 0
				}
			}
			margin := value.limit - value.current
			if !value.upper {
				margin = value.current - value.limit
			}
			continuous := worsening && previousWorsening[value.metric]
			metric := MetricTrend{Metric: value.metric, Change: change, ThresholdMargin: margin, Worsening: worsening, ConsecutiveWorsening: continuous}
			stage.Metrics = append(stage.Metrics, metric)
			if worsening {
				stage.Warnings = append(stage.Warnings, TrendWarning{Code: value.metric + "_worsening", Metric: value.metric, Change: change, ThresholdMargin: margin, ConsecutiveWorsening: continuous})
			}
			previousWorsening[value.metric] = worsening
		}
		result = append(result, stage)
	}
	return result
}
