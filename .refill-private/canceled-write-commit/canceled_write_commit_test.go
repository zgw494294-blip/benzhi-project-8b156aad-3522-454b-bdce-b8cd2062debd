package canceled_write_commit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stone-restoration-trial/internal/httpapi"
	"stone-restoration-trial/internal/store"
	"stone-restoration-trial/internal/workflow"
)

func TestCancelledCreateDoesNotCommit(t *testing.T) {
	repository, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("打开测试数据库: %v", err)
	}
	defer repository.Close()

	handler := httpapi.New(workflow.New(repository)).Handler()
	body := `{"idempotencyKey":"cancel-create-001","actor":"取消测试员","siteName":"钟楼","buildingArea":"北立面","stoneType":"砂岩","deteriorationSummary":"表层风化剥落","targetAppearance":"保持原有暖灰色调","acceptanceThresholds":{"maxColorDifference":3,"maxWaterAbsorption":5,"minAdhesionStrength":1}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/restoration-cases", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	requestContext, cancel := context.WithCancel(request.Context())
	cancel()
	request = request.WithContext(requestContext)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/v1/restoration-cases", nil))
	var result struct {
		Meta struct {
			Count int `json:"count"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(listResponse.Body).Decode(&result); err != nil {
		t.Fatalf("解析后续任务列表: %v", err)
	}
	if result.Meta.Count != 0 {
		t.Fatalf("已取消的创建请求仍返回 %d，后续列表 count=%d", response.Code, result.Meta.Count)
	}
}
