package domain

import (
	"fmt"
	"sort"
)

const MaxRetestRounds = 8

func PopulateDerivedEvidence(c *RestorationCase) error {
	for i := range c.Patches {
		root, round, err := RetestTrace(c, c.Patches[i].PatchID)
		if err != nil {
			return err
		}
		c.Patches[i].RootPatchID = root
		c.Patches[i].RetestRound = round
		c.Patches[i].Trends = BuildStageTrends(c.Patches[i], c.AcceptanceThresholds)
	}
	rootCodes := map[string]string{}
	for _, patch := range c.Patches {
		if patch.ParentPatchID == "" {
			rootCodes[patch.PatchID] = patch.PatchCode
		}
	}
	sort.SliceStable(c.Patches, func(i, j int) bool {
		left, right := c.Patches[i], c.Patches[j]
		if rootCodes[left.RootPatchID] != rootCodes[right.RootPatchID] {
			return rootCodes[left.RootPatchID] < rootCodes[right.RootPatchID]
		}
		if left.RetestRound != right.RetestRound {
			return left.RetestRound < right.RetestRound
		}
		return left.PatchCode < right.PatchCode
	})
	return nil
}

func RetestTrace(c *RestorationCase, patchID string) (string, int, error) {
	seen := map[string]bool{}
	currentID := patchID
	round := 0
	for {
		if seen[currentID] {
			return "", 0, fmt.Errorf("%w: 复验父子链包含循环", ErrInvalidState)
		}
		seen[currentID] = true
		patch, err := c.Patch(currentID)
		if err != nil {
			return "", 0, fmt.Errorf("%w: 复验父子链引用不存在的试验块", ErrInvalidState)
		}
		if patch.ParentPatchID == "" {
			return patch.PatchID, round, nil
		}
		round++
		currentID = patch.ParentPatchID
	}
}

func ValidateNextRetest(c *RestorationCase, failedPatchID string) (string, int, error) {
	patch, err := c.Patch(failedPatchID)
	if err != nil || patch.EvaluationResult == nil || patch.EvaluationResult.Conclusion != "failed" {
		return "", 0, fmt.Errorf("%w: 只有终期不合格试验块可以创建复验", ErrInvalidState)
	}
	for _, candidate := range c.Patches {
		if candidate.ParentPatchID == failedPatchID {
			return "", 0, fmt.Errorf("%w: 同一失败试验块已存在直接复验分支", ErrInvalidState)
		}
	}
	root, round, err := RetestTrace(c, failedPatchID)
	if err != nil {
		return "", 0, err
	}
	if round+1 > MaxRetestRounds {
		return "", 0, fmt.Errorf("%w: 复验轮次不能超过 %d 轮", ErrInvalidState, MaxRetestRounds)
	}
	return root, round + 1, nil
}

func HasPassingDescendant(c *RestorationCase, patchID string) bool {
	seen := map[string]bool{}
	var visit func(string) bool
	visit = func(parent string) bool {
		if seen[parent] {
			return false
		}
		seen[parent] = true
		for _, patch := range c.Patches {
			if patch.ParentPatchID != parent {
				continue
			}
			if patch.EvaluationResult != nil && patch.EvaluationResult.Conclusion == "passed" {
				return true
			}
			if visit(patch.PatchID) {
				return true
			}
		}
		return false
	}
	return visit(patchID)
}
