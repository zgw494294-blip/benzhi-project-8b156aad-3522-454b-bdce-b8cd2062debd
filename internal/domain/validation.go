package domain

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func ValidateThresholds(value Thresholds) error {
	if !finite(value.MaxColorDifference) || value.MaxColorDifference <= 0 || value.MaxColorDifference > 100 {
		return Invalid("acceptanceThresholds.maxColorDifference", "颜色差上限必须在 0 到 100 之间")
	}
	if !finite(value.MaxWaterAbsorption) || value.MaxWaterAbsorption <= 0 || value.MaxWaterAbsorption > 100 {
		return Invalid("acceptanceThresholds.maxWaterAbsorption", "吸水率上限必须在 0 到 100 之间")
	}
	if !finite(value.MinAdhesionStrength) || value.MinAdhesionStrength <= 0 || value.MinAdhesionStrength > 1000 {
		return Invalid("acceptanceThresholds.minAdhesionStrength", "附着强度下限必须在 0 到 1000 之间")
	}
	return nil
}

func ValidateFormula(formula FormulaRevision) error {
	if len(formula.Ingredients) == 0 {
		return Invalid("ingredients", "配方至少需要一种原料")
	}
	total := 0.0
	seen := map[string]bool{}
	for i, ingredient := range formula.Ingredients {
		name := strings.TrimSpace(ingredient.Name)
		if name == "" {
			return Invalid(fmt.Sprintf("ingredients[%d].name", i), "原料名称不能为空")
		}
		if seen[name] {
			return Invalid("ingredients", "原料名称不能重复")
		}
		seen[name] = true
		if !finite(ingredient.Percentage) || ingredient.Percentage <= 0 || ingredient.Percentage > 100 {
			return Invalid(fmt.Sprintf("ingredients[%d].percentage", i), "原料比例必须在 0 到 100 之间")
		}
		total += ingredient.Percentage
	}
	if math.Abs(total-100) > 0.01 {
		return Invalid("ingredients", "原料比例合计必须为 100")
	}
	if strings.TrimSpace(formula.ApplicationMethod) == "" {
		return Invalid("applicationMethod", "施工工法不能为空")
	}
	if strings.TrimSpace(formula.SubstrateConditions) == "" {
		return Invalid("substrateConditions", "基材条件不能为空")
	}
	if strings.TrimSpace(formula.CreatedBy) == "" {
		return Invalid("createdBy", "编制人不能为空")
	}
	return nil
}

func ValidateObservation(observation Observation, now time.Time) error {
	if StageIndex(observation.Stage) < 0 {
		return Invalid("stage", "养护阶段无效")
	}
	if !finite(observation.ColorDifference) || observation.ColorDifference < 0 || observation.ColorDifference > 100 {
		return Invalid("colorDifference", "颜色差必须在 0 到 100 之间")
	}
	if !finite(observation.WaterAbsorption) || observation.WaterAbsorption < 0 || observation.WaterAbsorption > 100 {
		return Invalid("waterAbsorption", "吸水率必须在 0 到 100 之间")
	}
	if !finite(observation.AdhesionStrength) || observation.AdhesionStrength < 0 || observation.AdhesionStrength > 1000 {
		return Invalid("adhesionStrength", "附着强度必须在 0 到 1000 之间")
	}
	if observation.ObservedAt.IsZero() {
		return Invalid("observedAt", "观测时间不能为空")
	}
	if observation.ObservedAt.After(now.Add(5 * time.Minute)) {
		return Invalid("observedAt", "观测时间不能晚于当前时间")
	}
	if strings.TrimSpace(observation.EvidenceSummary) == "" {
		return Invalid("evidenceSummary", "证据摘要不能为空")
	}
	if strings.TrimSpace(observation.RecordedBy) == "" {
		return Invalid("recordedBy", "记录人不能为空")
	}
	for _, defect := range observation.SurfaceDefects {
		if strings.TrimSpace(defect) == "" {
			return Invalid("surfaceDefects", "表面缺陷描述不能为空")
		}
	}
	return nil
}

func ValidateObservationTimeline(patch *TrialPatch, observation Observation) error {
	if !observation.ObservedAt.After(patch.CreatedAt) {
		return Invalid("observedAt", "观测时间必须晚于试验块创建时间")
	}
	if len(patch.Observations) > 0 {
		previous := patch.Observations[len(patch.Observations)-1]
		if !observation.ObservedAt.After(previous.ObservedAt) {
			return Invalid("observedAt", "观测时间必须严格晚于前一养护阶段")
		}
	}
	return nil
}

func ValidateBaselineRevision(deterioration, target string, thresholds Thresholds, reason string) error {
	fields := []struct{ name, value string }{
		{"deteriorationSummary", deterioration},
		{"targetAppearance", target},
		{"reason", reason},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return Invalid(field.name, "必填内容不能为空")
		}
		if len([]rune(field.value)) > 1000 {
			return Invalid(field.name, "内容过长")
		}
	}
	return ValidateThresholds(thresholds)
}

func ValidatePatchCodes(c *RestorationCase, values []string) ([]string, error) {
	if len(values) < 2 || len(values) > 50 {
		return nil, Invalid("patchCodes", "成组登记必须包含 2 至 50 个试验块编号")
	}
	normalized := make([]string, len(values))
	seen := map[string]bool{}
	for index, value := range values {
		code := strings.TrimSpace(value)
		field := fmt.Sprintf("patchCodes[%d]", index)
		if code == "" || len([]rune(code)) > 80 {
			return nil, Invalid(field, "试验块编号不能为空且不能超过 80 个字符")
		}
		if seen[code] {
			return nil, Invalid(field, "组内试验块编号重复: "+code)
		}
		if c.HasPatchCode(code) {
			return nil, Invalid(field, "试验块编号已被占用: "+code)
		}
		seen[code] = true
		normalized[index] = code
	}
	return normalized, nil
}

func ValidateCaseText(c RestorationCase) error {
	fields := []struct{ name, value string }{
		{"siteName", c.SiteName}, {"buildingArea", c.BuildingArea}, {"stoneType", c.StoneType},
		{"deteriorationSummary", c.DeteriorationSummary}, {"targetAppearance", c.TargetAppearance},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return Invalid(field.name, "必填内容不能为空")
		}
		if len([]rune(field.value)) > 1000 {
			return Invalid(field.name, "内容过长")
		}
	}
	return ValidateThresholds(c.AcceptanceThresholds)
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
