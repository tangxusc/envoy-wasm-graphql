package federation

import (
	"testing"

	federationtypes "envoy-wasm-graphql-federation/pkg/types"
	"envoy-wasm-graphql-federation/pkg/utils"
)

func TestDirectiveParser_ParseKeyDirective(t *testing.T) {
	logger := utils.NewLogger("test")
	parser := NewDirectiveParser(logger)

	tests := []struct {
		name      string
		directive string
		expected  *federationtypes.KeyDirective
		wantErr   bool
	}{
		{
			name:      "simple key directive",
			directive: `@key(fields: "id")`,
			expected: &federationtypes.KeyDirective{
				Fields:     "id",
				Resolvable: true,
			},
			wantErr: false,
		},
		{
			name:      "key directive with multiple fields",
			directive: `@key(fields: "id username")`,
			expected: &federationtypes.KeyDirective{
				Fields:     "id username",
				Resolvable: true,
			},
			wantErr: false,
		},
		{
			name:      "key directive with resolvable false",
			directive: `@key(fields: "id", resolvable: false)`,
			expected: &federationtypes.KeyDirective{
				Fields:     "id",
				Resolvable: false,
			},
			wantErr: false,
		},
		{
			name:      "invalid directive format",
			directive: `@key(invalid)`,
			expected:  nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseKeyDirective(tt.directive)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseKeyDirective() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseKeyDirective() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("ParseKeyDirective() returned nil result")
				return
			}

			if result.Fields != tt.expected.Fields {
				t.Errorf("ParseKeyDirective() fields = %v, expected %v", result.Fields, tt.expected.Fields)
			}

			if result.Resolvable != tt.expected.Resolvable {
				t.Errorf("ParseKeyDirective() resolvable = %v, expected %v", result.Resolvable, tt.expected.Resolvable)
			}
		})
	}
}

func TestDirectiveParser_ParseExternalDirective(t *testing.T) {
	logger := utils.NewLogger("test")
	parser := NewDirectiveParser(logger)

	tests := []struct {
		name      string
		directive string
		expected  *federationtypes.ExternalDirective
		wantErr   bool
	}{
		{
			name:      "simple external directive",
			directive: `@external`,
			expected: &federationtypes.ExternalDirective{
				Reason: "",
			},
			wantErr: false,
		},
		{
			name:      "external directive with reason",
			directive: `@external(reason: "provided by user service")`,
			expected: &federationtypes.ExternalDirective{
				Reason: "provided by user service",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseExternalDirective(tt.directive)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseExternalDirective() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseExternalDirective() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("ParseExternalDirective() returned nil result")
				return
			}

			if result.Reason != tt.expected.Reason {
				t.Errorf("ParseExternalDirective() reason = %v, expected %v", result.Reason, tt.expected.Reason)
			}
		})
	}
}

func TestDirectiveParser_ParseRequiresDirective(t *testing.T) {
	logger := utils.NewLogger("test")
	parser := NewDirectiveParser(logger)

	tests := []struct {
		name      string
		directive string
		expected  *federationtypes.RequiresDirective
		wantErr   bool
	}{
		{
			name:      "simple requires directive",
			directive: `@requires(fields: "email")`,
			expected: &federationtypes.RequiresDirective{
				Fields: "email",
			},
			wantErr: false,
		},
		{
			name:      "requires directive with multiple fields",
			directive: `@requires(fields: "email username")`,
			expected: &federationtypes.RequiresDirective{
				Fields: "email username",
			},
			wantErr: false,
		},
		{
			name:      "invalid directive format",
			directive: `@requires(invalid)`,
			expected:  nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseRequiresDirective(tt.directive)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseRequiresDirective() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseRequiresDirective() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("ParseRequiresDirective() returned nil result")
				return
			}

			if result.Fields != tt.expected.Fields {
				t.Errorf("ParseRequiresDirective() fields = %v, expected %v", result.Fields, tt.expected.Fields)
			}
		})
	}
}

func TestDirectiveParser_ParseProvidesDirective(t *testing.T) {
	logger := utils.NewLogger("test")
	parser := NewDirectiveParser(logger)

	tests := []struct {
		name      string
		directive string
		expected  *federationtypes.ProvidesDirective
		wantErr   bool
	}{
		{
			name:      "simple provides directive",
			directive: `@provides(fields: "username")`,
			expected: &federationtypes.ProvidesDirective{
				Fields: "username",
			},
			wantErr: false,
		},
		{
			name:      "provides directive with multiple fields",
			directive: `@provides(fields: "username email")`,
			expected: &federationtypes.ProvidesDirective{
				Fields: "username email",
			},
			wantErr: false,
		},
		{
			name:      "invalid directive format",
			directive: `@provides(invalid)`,
			expected:  nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseProvidesDirective(tt.directive)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseProvidesDirective() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseProvidesDirective() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("ParseProvidesDirective() returned nil result")
				return
			}

			if result.Fields != tt.expected.Fields {
				t.Errorf("ParseProvidesDirective() fields = %v, expected %v", result.Fields, tt.expected.Fields)
			}
		})
	}
}

func TestDirectiveParser_ValidateDirectives(t *testing.T) {
	logger := utils.NewLogger("test")
	parser := NewDirectiveParser(logger)

	tests := []struct {
		name       string
		directives *federationtypes.EntityDirectives
		wantErr    bool
	}{
		{
			name: "valid directives with key",
			directives: &federationtypes.EntityDirectives{
				Keys: []federationtypes.KeyDirective{
					{Fields: "id", Resolvable: true},
				},
			},
			wantErr: false,
		},
		{
			name: "valid external field",
			directives: &federationtypes.EntityDirectives{
				External: &federationtypes.ExternalDirective{},
			},
			wantErr: false,
		},
		{
			name: "invalid requires without external",
			directives: &federationtypes.EntityDirectives{
				Requires: &federationtypes.RequiresDirective{
					Fields: "email",
				},
			},
			wantErr: true,
		},
		{
			name: "valid requires with external",
			directives: &federationtypes.EntityDirectives{
				External: &federationtypes.ExternalDirective{},
				Requires: &federationtypes.RequiresDirective{
					Fields: "email",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ValidateDirectives(tt.directives)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateDirectives() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDirectives() unexpected error: %v", err)
				}
			}
		})
	}
}
