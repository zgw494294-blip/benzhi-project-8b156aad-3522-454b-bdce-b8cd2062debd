package conflictretrylostupdate

import (
	"context"
	"errors"
	"stone-restoration-trial/internal/domain"
	"stone-restoration-trial/internal/store"
	"stone-restoration-trial/internal/workflow"
	"testing"
)

func TestConcurrentStaleCommandsKeepVersionConflict(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := workflow.New(repository)
	created, _, err := service.CreateCase(ctx, workflow.CreateCaseCommand{
		IdempotencyKey:       "private-create-001",
		Actor:                "并发复现员",
		SiteName:             "古建东门",
		BuildingArea:         "台基转角",
		StoneType:            "青石",
		DeteriorationSummary: "表面粉化",
		TargetAppearance:     "色泽与原石协调",
		AcceptanceThresholds: domain.Thresholds{MaxColorDifference: 3, MaxWaterAbsorption: 5, MinAdhesionStrength: 1},
	})
	if err != nil {
		t.Fatal(err)
	}

	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	results := make(chan error, 2)
	for index := 0; index < 2; index++ {
		index := index
		go func() {
			ready <- struct{}{}
			<-start
			_, _, updateErr := service.AddFormula(ctx, created.CaseID, workflow.AddFormulaCommand{
				CommandMeta:         workflow.CommandMeta{ExpectedVersion: created.Version, IdempotencyKey: []string{"stale-formula-a", "stale-formula-b"}[index], Actor: "并发复现员"},
				Ingredients:         []domain.Ingredient{{Name: []string{"石灰", "矿粉"}[index], Percentage: 100}},
				ApplicationMethod:   "薄层刮涂",
				SubstrateConditions: "基材清洁并预湿",
				ChangeReason:        "并发试配",
			})
			results <- updateErr
		}()
	}
	<-ready
	<-ready
	close(start)

	successes, conflicts := 0, 0
	for index := 0; index < 2; index++ {
		result := <-results
		switch {
		case result == nil:
			successes++
		case errors.Is(result, domain.ErrConflict):
			conflicts++
		default:
			t.Fatalf("并发写返回了非预期错误: %v", result)
		}
	}
	stored, err := service.GetCase(ctx, created.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if successes != 1 || conflicts != 1 || stored.Version != created.Version+1 || len(stored.Formulas) != 1 {
		t.Fatalf("陈旧写请求未保留版本冲突: success=%d conflict=%d version=%d formulas=%d", successes, conflicts, stored.Version, len(stored.Formulas))
	}
}
