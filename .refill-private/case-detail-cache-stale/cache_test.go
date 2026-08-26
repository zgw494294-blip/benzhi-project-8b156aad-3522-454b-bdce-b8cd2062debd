package case_detail_cache_stale_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"stone-restoration-trial/internal/domain"
	"stone-restoration-trial/internal/httpapi"
	"stone-restoration-trial/internal/store"
	"stone-restoration-trial/internal/workflow"
	"testing"
)

type caseEnvelope struct {
	Data domain.RestorationCase `json:"data"`
}

func TestDetailCacheInvalidatesAfterFormulaWrite(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	service := workflow.New(repository)
	created, _, err := service.CreateCase(ctx, workflow.CreateCaseCommand{
		IdempotencyKey:       "cache-create-001",
		Actor:                "缓存复现员",
		SiteName:             "古塔",
		BuildingArea:         "首层外墙",
		StoneType:            "砂岩",
		DeteriorationSummary: "表层粉化",
		TargetAppearance:     "色泽与原石协调",
		AcceptanceThresholds: domain.Thresholds{
			MaxColorDifference:  3,
			MaxWaterAbsorption:  5,
			MinAdhesionStrength: 1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(service).Handler()

	before := getCase(t, handler, created.CaseID)
	if before.Version != 1 || len(before.Formulas) != 0 {
		t.Fatalf("初始详情异常: version=%d formulas=%d", before.Version, len(before.Formulas))
	}

	body, err := json.Marshal(workflow.AddFormulaCommand{
		CommandMeta: workflow.CommandMeta{
			ExpectedVersion: 1,
			IdempotencyKey:  "cache-formula-001",
			Actor:           "缓存复现员",
		},
		Ingredients:         []domain.Ingredient{{Name: "石灰", Percentage: 100}},
		ApplicationMethod:   "薄层刮涂",
		SubstrateConditions: "清洁并预湿",
		ChangeReason:        "初始试配",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/restoration-cases/"+created.CaseID+"/formulas", bytes.NewReader(body))
	writeRequest.Header.Set("Content-Type", "application/json")
	writeResponse := httptest.NewRecorder()
	handler.ServeHTTP(writeResponse, writeRequest)
	if writeResponse.Code != http.StatusOK {
		t.Fatalf("新增配方失败: status=%d body=%s", writeResponse.Code, writeResponse.Body.String())
	}

	after := getCase(t, handler, created.CaseID)
	if after.Version != 2 || len(after.Formulas) != 1 {
		t.Fatalf("配方写入后详情缓存未失效: version=%d formulas=%d", after.Version, len(after.Formulas))
	}
}

func getCase(t *testing.T, handler http.Handler, caseID string) domain.RestorationCase {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/restoration-cases/"+caseID, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("查询详情失败: status=%d body=%s", response.Code, response.Body.String())
	}
	var envelope caseEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}
