package service

import (
	"reflect"
	"testing"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
)

func TestToJSON(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want model.JSON
	}{
		{"nil", nil, nil},
		{"string", "hello", model.JSON(`"hello"`)},
		{"int", 42, model.JSON(`42`)},
		{"map", map[string]int{"a": 1}, model.JSON(`{"a":1}`)},
		{"marshal fail", make(chan int), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toJSON(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("toJSON(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestMergeModelName(t *testing.T) {
	tests := []struct {
		name string
		p    map[string]any
		m    string
		want map[string]any
	}{
		{
			name: "nil map with name",
			p:    nil,
			m:    "gpt-4",
			want: map[string]any{"_model": "gpt-4"},
		},
		{
			name: "empty name",
			p:    map[string]any{"a": 1},
			m:    "",
			want: map[string]any{"a": 1},
		},
		{
			name: "overwrite existing",
			p:    map[string]any{"_model": "old"},
			m:    "new",
			want: map[string]any{"_model": "new"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeModelName(tt.p, tt.m)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("mergeModelName(%v, %q) = %v, want %v", tt.p, tt.m, got, tt.want)
			}
		})
	}
}

func TestReadModelName(t *testing.T) {
	tests := []struct {
		name string
		m    *model.Model
		want string
	}{
		{
			name: "nil model",
			m:    nil,
			want: "",
		},
		{
			name: "model with _model in params",
			m: &model.Model{
				Code:          "code-fallback",
				DefaultParams: model.JSON(`{"_model":"custom-name"}`),
			},
			want: "custom-name",
		},
		{
			name: "model without _model falls back to code",
			m: &model.Model{
				Code:          "code-fallback",
				DefaultParams: model.JSON(`{"other":"value"}`),
			},
			want: "code-fallback",
		},
		{
			name: "empty params falls back to code",
			m: &model.Model{
				Code:          "code-fallback",
				DefaultParams: nil,
			},
			want: "code-fallback",
		},
		{
			name: "_model is not string falls back to code",
			m: &model.Model{
				Code:          "code-fallback",
				DefaultParams: model.JSON(`{"_model":123}`),
			},
			want: "code-fallback",
		},
		{
			name: "_model is empty string falls back to code",
			m: &model.Model{
				Code:          "code-fallback",
				DefaultParams: model.JSON(`{"_model":""}`),
			},
			want: "code-fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readModelName(tt.m)
			if got != tt.want {
				t.Fatalf("readModelName(%v) = %q, want %q", tt.m, got, tt.want)
			}
		})
	}
}
