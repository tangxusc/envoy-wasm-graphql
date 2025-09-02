package federation

import (
	"context"
	"testing"

	federationtypes "envoy-wasm-graphql-federation/pkg/types"
	"envoy-wasm-graphql-federation/pkg/utils"
)

// 模拟服务调用器
type mockServiceCaller struct {
	responses map[string]*federationtypes.ServiceResponse
}

func (m *mockServiceCaller) Call(ctx context.Context, call *federationtypes.ServiceCall) (*federationtypes.ServiceResponse, error) {
	if response, exists := m.responses[call.Service.Name]; exists {
		return response, nil
	}

	return &federationtypes.ServiceResponse{
		Data: map[string]interface{}{
			"_entities": []interface{}{
				map[string]interface{}{
					"__typename": "User",
					"id":         "1",
					"username":   "testuser",
				},
			},
		},
		Service: call.Service.Name,
	}, nil
}

func (m *mockServiceCaller) CallBatch(ctx context.Context, calls []*federationtypes.ServiceCall) ([]*federationtypes.ServiceResponse, error) {
	var responses []*federationtypes.ServiceResponse
	for _, call := range calls {
		response, err := m.Call(ctx, call)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func (m *mockServiceCaller) IsHealthy(ctx context.Context, service *federationtypes.ServiceConfig) bool {
	return true
}

func TestEntityResolver_ResolveEntity(t *testing.T) {
	logger := utils.NewLogger("test")
	caller := &mockServiceCaller{
		responses: make(map[string]*federationtypes.ServiceResponse),
	}
	resolver := NewEntityResolver(logger, caller)

	representation := federationtypes.RepresentationRequest{
		TypeName: "User",
		Representation: map[string]interface{}{
			"id": "1",
		},
	}

	result, err := resolver.ResolveEntity(context.Background(), "user-service", representation)
	if err != nil {
		t.Fatalf("ResolveEntity() error = %v", err)
	}

	if result == nil {
		t.Error("ResolveEntity() returned nil result")
	}
}

func TestEntityResolver_ResolveBatchEntities(t *testing.T) {
	logger := utils.NewLogger("test")
	caller := &mockServiceCaller{
		responses: make(map[string]*federationtypes.ServiceResponse),
	}
	resolver := NewEntityResolver(logger, caller)

	representations := []federationtypes.RepresentationRequest{
		{
			TypeName: "User",
			Representation: map[string]interface{}{
				"id": "1",
			},
		},
		{
			TypeName: "User",
			Representation: map[string]interface{}{
				"id": "2",
			},
		},
	}

	results, err := resolver.ResolveBatchEntities(context.Background(), "user-service", representations)
	if err != nil {
		t.Fatalf("ResolveBatchEntities() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("ResolveBatchEntities() should return results")
	}
}

func TestEntityResolver_ValidateRepresentation(t *testing.T) {
	logger := utils.NewLogger("test")
	resolver := NewEntityResolver(logger, nil)

	entity := &federationtypes.FederatedEntity{
		TypeName: "User",
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

	tests := []struct {
		name           string
		representation federationtypes.RepresentationRequest
		wantErr        bool
	}{
		{
			name: "valid representation",
			representation: federationtypes.RepresentationRequest{
				TypeName: "User",
				Representation: map[string]interface{}{
					"id": "1",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid typename",
			representation: federationtypes.RepresentationRequest{
				TypeName: "Product",
				Representation: map[string]interface{}{
					"id": "1",
				},
			},
			wantErr: true,
		},
		{
			name: "missing key field",
			representation: federationtypes.RepresentationRequest{
				TypeName: "User",
				Representation: map[string]interface{}{
					"username": "testuser",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolver.ValidateRepresentation(entity, tt.representation)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateRepresentation() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ValidateRepresentation() unexpected error: %v", err)
				}
			}
		})
	}
}
