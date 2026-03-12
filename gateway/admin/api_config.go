package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

// handleGetConfig 获取配置（脱敏）
func (h *AdminHandler) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if h.config == nil {
		writeError(w, http.StatusInternalServerError, "config not available")
		return
	}

	// 序列化配置
	data, err := json.Marshal(h.config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal config")
		return
	}

	// 反序列化为 map 以便脱敏
	var configMap map[string]interface{}
	if err := json.Unmarshal(data, &configMap); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process config")
		return
	}

	// 脱敏敏感字段
	maskSensitiveFields(configMap)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config":      configMap,
		"config_path": h.configPath,
	})
}

// handlePutConfig 更新配置
func (h *AdminHandler) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	if h.configPath == "" {
		writeError(w, http.StatusBadRequest, "config path not set")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	// 解析提交的 JSON
	var newMap map[string]interface{}
	if err := json.Unmarshal(body, &newMap); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// 读取磁盘上的原始配置，用于还原脱敏字段
	originalData, err := os.ReadFile(h.configPath)
	if err == nil {
		var originalMap map[string]interface{}
		if json.Unmarshal(originalData, &originalMap) == nil {
			restoreSensitiveFields(newMap, originalMap)
		}
	}

	// 序列化合并后的结果
	merged, err := json.MarshalIndent(newMap, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal config: "+err.Error())
		return
	}

	// 写入配置文件
	if err := os.WriteFile(h.configPath, merged, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write config file: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "updated",
		"message": "config saved, restart to apply changes",
	})
}

// restoreSensitiveFields 递归还原脱敏字段：如果新值以 "****" 结尾，则从原始 map 恢复
func restoreSensitiveFields(newMap, originalMap map[string]interface{}) {
	for k, v := range newMap {
		switch val := v.(type) {
		case string:
			if strings.HasSuffix(val, "****") {
				if origVal, ok := originalMap[k]; ok {
					if origStr, ok := origVal.(string); ok {
						newMap[k] = origStr
					}
				}
			}
		case map[string]interface{}:
			if origNested, ok := originalMap[k].(map[string]interface{}); ok {
				restoreSensitiveFields(val, origNested)
			}
		}
	}
}

// maskSensitiveFields 递归脱敏敏感字段
func maskSensitiveFields(m map[string]interface{}) {
	sensitiveKeys := []string{"key", "token", "secret", "password", "api_key", "apikey", "auth_token"}
	for k, v := range m {
		kLower := strings.ToLower(k)
		for _, sk := range sensitiveKeys {
			if strings.Contains(kLower, sk) {
				if s, ok := v.(string); ok && len(s) > 4 {
					m[k] = s[:4] + "****"
				} else if _, ok := v.(string); ok {
					m[k] = "****"
				}
				break
			}
		}
		// 递归处理嵌套 map
		if nested, ok := v.(map[string]interface{}); ok {
			maskSensitiveFields(nested)
		}
	}
}
