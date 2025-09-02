package federation

import (
	"testing"

	federationtypes "envoy-wasm-graphql-federation/pkg/types"
	"envoy-wasm-graphql-federation/pkg/utils"
)

func TestFederatedPlanner_PlanEntityResolution(t *testing.T) {
	logger := utils.NewLogger("test")
	planner := NewFederatedPlanner(logger)

	// 创建测试实体
	entities := []federationtypes.FederatedEntity{
		{
			TypeName:    "User",
			ServiceName: "user-service",
			Directives: federationtypes.EntityDirectives{
				Keys: []federationtypes.KeyDirective{
					{Fields: "id", Resolvable: true},
				},
			},
			Fields: []federationtypes.FederatedField{
				{Name: "id", Type: "ID"},
				{Name: "username", Type: "String"},
			},
		},
		{
			TypeName:    "Product",
			ServiceName: "product-service",
			Directives: federationtypes.EntityDirectives{
				Keys: []federationtypes.KeyDirective{
					{Fields: "id", Resolvable: true},
				},
			},
			Fields: []federationtypes.FederatedField{
				{Name: "id", Type: "ID"},
				{Name: "name", Type: "String"},
			},
		},
	}

	// 创建测试查询
	query := &federationtypes.ParsedQuery{
		Operation:  "GetUserAndProduct",
		Variables:  make(map[string]interface{}),
		Fragments:  make(map[string]interface{}),
		Complexity: 5,
		Depth:      2,
	}

	plan, err := planner.PlanEntityResolution(entities, query)
	if err != nil {
		t.Fatalf("PlanEntityResolution() error = %v", err)
	}

	if plan == nil {
		t.Fatal("PlanEntityResolution() returned nil plan")
	}

	// 验证计划包含实体解析
	if len(plan.Entities) == 0 {
		t.Error("PlanEntityResolution() plan should contain entity resolutions")
	}

	// 验证计划包含所需服务
	expectedServices := []string{"user-service", "product-service"}
	if len(plan.RequiredServices) < len(expectedServices) {
		t.Errorf("PlanEntityResolution() expected at least %d required services, got %d", len(expectedServices), len(plan.RequiredServices))
	}
}

func TestFederatedPlanner_BuildRepresentationQuery(t *testing.T) {
	logger := utils.NewLogger("test")
	planner := NewFederatedPlanner(logger)

	entity := &federationtypes.FederatedEntity{
		TypeName:    "User",
		ServiceName: "user-service",
		Directives: federationtypes.EntityDirectives{
			Keys: []federationtypes.KeyDirective{
				{Fields: "id", Resolvable: true},
			},
		},
		Fields: []federationtypes.FederatedField{
			{Name: "id", Type: "ID"},
			{Name: "username", Type: "String"},
		},
	}

	representations := []federationtypes.RepresentationRequest{
		{
			TypeName: "User",
			Representation: map[string]interface{}{
				"id": "1",
			},
		},
	}

	query, err := planner.BuildRepresentationQuery(entity, representations)
	if err != nil {
		t.Fatalf("BuildRepresentationQuery() error = %v", err)
	}

	if query == "" {
		t.Error("BuildRepresentationQuery() returned empty query")
	}

	// 验证查询包含 _entities
	if !contains(query, "_entities") {
		t.Error("BuildRepresentationQuery() query should contain '_entities'")
	}

	// 验证查询包含类型名
	if !contains(query, "User") {
		t.Error("BuildRepresentationQuery() query should contain type name 'User'")
	}
}

func TestFederatedPlanner_AnalyzeDependencies(t *testing.T) {
	logger := utils.NewLogger("test")
	planner := NewFederatedPlanner(logger)

	entities := []federationtypes.FederatedEntity{
		{
			TypeName:    "User",
			ServiceName: "user-service",
			Fields: []federationtypes.FederatedField{
				{Name: "id", Type: "ID"},
				{Name: "email", Type: "String"},
			},
		},
		{
			TypeName:    "User",
			ServiceName: "profile-service",
			Fields: []federationtypes.FederatedField{
				{
					Name: "email",
					Type: "String",
					Directives: federationtypes.EntityDirectives{
						External: &federationtypes.ExternalDirective{},
					},
				},
				{
					Name: "profile",
					Type: "Profile",
					Directives: federationtypes.EntityDirectives{
						Requires: &federationtypes.RequiresDirective{
							Fields: "email",
						},
					},
				},
			},
		},
	}

	order, err := planner.AnalyzeDependencies(entities)
	if err != nil {
		t.Fatalf("AnalyzeDependencies() error = %v", err)
	}

	if len(order) == 0 {
		t.Error("AnalyzeDependencies() should return dependency order")
	}

	// 验证基础服务先于依赖服务
	// profile-service 依赖 user-service 提供的 email 字段
	// 因此 user-service 应该在 profile-service 之前
	userServiceIndex := indexOf(order, "user-service")
	profileServiceIndex := indexOf(order, "profile-service")

	if userServiceIndex != -1 && profileServiceIndex != -1 {
		if userServiceIndex > profileServiceIndex {
			t.Errorf("AnalyzeDependencies() user-service should come before profile-service, got order: %v", order)
		}
	} else {
		t.Errorf("Both services should be in the dependency order, got: %v", order)
	}
}

func TestFederatedPlanner_OptimizeFederationPlan(t *testing.T) {
	logger := utils.NewLogger("test")
	planner := NewFederatedPlanner(logger)

	originalPlan := &federationtypes.FederationPlan{
		Entities: []federationtypes.EntityResolution{
			{
				TypeName:    "User",
				ServiceName: "user-service",
				KeyFields:   []string{"id"},
				Query:       "{ id username }",
			},
			{
				TypeName:    "Product",
				ServiceName: "product-service",
				KeyFields:   []string{"id"},
				Query:       "{ id name }",
			},
		},
		RequiredServices: []string{"user-service", "product-service"},
		DependencyOrder:  []string{"user-service", "product-service"},
	}

	optimizedPlan, err := planner.OptimizeFederationPlan(originalPlan)
	if err != nil {
		t.Fatalf("OptimizeFederationPlan() error = %v", err)
	}

	if optimizedPlan == nil {
		t.Fatal("OptimizeFederationPlan() returned nil plan")
	}

	// 验证优化后的计划仍包含必要信息
	if len(optimizedPlan.Entities) == 0 {
		t.Error("OptimizeFederationPlan() optimized plan should contain entities")
	}

	if len(optimizedPlan.RequiredServices) == 0 {
		t.Error("OptimizeFederationPlan() optimized plan should contain required services")
	}
}

// 辅助函数

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
