package service

import (
	"encoding/json"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
)

func toJSON(v any) model.JSON {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return model.JSON(b)
}

// mergeModelName 把 model_name 合并进 default_params,作为后续 adapter.modelName 的来源
func mergeModelName(p map[string]any, name string) map[string]any {
	if p == nil {
		p = map[string]any{}
	}
	if name != "" {
		p["_model"] = name
	}
	return p
}

// readModelName 从 default_params._model 读取上游模型名,缺省回退到 model.Code
func readModelName(m *model.Model) string {
	if m == nil {
		return ""
	}
	mp, err := m.DefaultParams.AsMap()
	if err == nil && mp != nil {
		if v, ok := mp["_model"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return m.Code
}
