package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"stone-restoration-trial/internal/domain"
	"stone-restoration-trial/internal/store"
	"stone-restoration-trial/internal/workflow"
	"strings"
	"testing"
)

func TestWorkbenchAndCreateEndpoint(t *testing.T) {
	repository, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	server := httptest.NewServer(New(workflow.New(repository)).Handler())
	defer server.Close()
	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "石材修复试配验证") {
		t.Fatalf("工作台不可达: %d %s", response.StatusCode, body)
	}
	assetResponse, err := http.Get(server.URL + "/assets/features.css")
	if err != nil {
		t.Fatal(err)
	}
	assetResponse.Body.Close()
	if assetResponse.StatusCode != http.StatusOK {
		t.Fatalf("扩展工作台样式不可达: %d", assetResponse.StatusCode)
	}
	payload := map[string]any{"idempotencyKey": "http-create-0001", "actor": "接口测试员", "siteName": "牌坊", "buildingArea": "柱础", "stoneType": "青石", "deteriorationSummary": "表层粉化", "targetAppearance": "色调一致", "acceptanceThresholds": map[string]float64{"maxColorDifference": 3, "maxWaterAbsorption": 5, "minAdhesionStrength": 1}}
	data, _ := json.Marshal(payload)
	response, err = http.Post(server.URL+"/api/v1/restoration-cases", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		result, _ := io.ReadAll(response.Body)
		t.Fatalf("创建端点返回 %d: %s", response.StatusCode, result)
	}
	if response.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("缺少安全响应头")
	}
}

func TestStableValidationError(t *testing.T) {
	repository, _ := store.Open(context.Background(), ":memory:")
	defer repository.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/restoration-cases", strings.NewReader(`{"unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	New(workflow.New(repository)).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "validation_error") {
		t.Fatalf("校验错误不稳定: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestExtensionEndpointsAndStructuredReadiness(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := workflow.New(repository)
	c, _, err := service.CreateCase(ctx, workflow.CreateCaseCommand{IdempotencyKey: "http-extension-create", Actor: "接口测试员", SiteName: "城楼", BuildingArea: "台基", StoneType: "砂岩", DeteriorationSummary: "片状剥落", TargetAppearance: "色泽协调", AcceptanceThresholds: domain.Thresholds{MaxColorDifference: 3, MaxWaterAbsorption: 5, MinAdhesionStrength: 1}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(service).Handler())
	defer server.Close()
	response := postTestJSON(t, server.URL+"/api/v1/restoration-cases/"+c.CaseID+"/baseline-revisions", map[string]any{"expectedVersion": 1, "idempotencyKey": "http-baseline-001", "actor": "接口测试员", "deteriorationSummary": "片状剥落与粉化", "targetAppearance": "色泽与纹理协调", "acceptanceThresholds": map[string]float64{"maxColorDifference": 2.5, "maxWaterAbsorption": 5, "minAdhesionStrength": 1}, "reason": "纠正建档录入"})
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("基线修订端点返回 %d: %s", response.StatusCode, body)
	}
	response.Body.Close()
	c, _, err = service.AddFormula(ctx, c.CaseID, workflow.AddFormulaCommand{CommandMeta: workflow.CommandMeta{ExpectedVersion: 2, IdempotencyKey: "http-formula-001", Actor: "接口测试员"}, Ingredients: []domain.Ingredient{{Name: "石灰", Percentage: 100}}, ApplicationMethod: "刮涂", SubstrateConditions: "清洁", ChangeReason: "初始试配"})
	if err != nil {
		t.Fatal(err)
	}
	response = postTestJSON(t, server.URL+"/api/v1/restoration-cases/"+c.CaseID+"/patches", map[string]any{"expectedVersion": 3, "idempotencyKey": "http-batch-0001", "actor": "接口测试员", "formulaID": c.Formulas[0].FormulaID, "patchCodes": []string{"H-01", "H-02"}})
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("批量登记端点返回 %d: %s", response.StatusCode, body)
	}
	response.Body.Close()
	response = postTestJSON(t, server.URL+"/api/v1/restoration-cases/"+c.CaseID+"/submit-review", map[string]any{"expectedVersion": 4, "idempotencyKey": "http-submit-blocked", "actor": "接口测试员"})
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(body), "review_blocked") || !strings.Contains(string(body), "missing_observation_stage") {
		t.Fatalf("送审阻断响应不完整: %d %s", response.StatusCode, body)
	}
}

func postTestJSON(t *testing.T, url string, payload any) *http.Response {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
