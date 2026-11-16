package toolCalling

import (
	"AIxVuln/misc"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

func NormalizeAndValidateToolArguments(raw string, schema map[string]interface{}) (map[string]interface{}, string, error) {
	trimmed := strings.TrimSpace(raw)
	var decoded interface{}
	switch trimmed {
	case "", "null":
		decoded = map[string]interface{}{}
	default:
		if err := misc.UnmarshalJSONLenient(trimmed, &decoded); err != nil {
			return nil, "", err
		}
	}
	if decoded == nil {
		decoded = map[string]interface{}{}
	}
	args, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, "", fmt.Errorf("tool arguments root must be a JSON object")
	}
	if err := ValidateToolArgumentsAgainstSchema(args, schema); err != nil {
		return nil, "", err
	}
	normalized, err := json.Marshal(args)
	if err != nil {
		return nil, "", err
	}
	return args, string(normalized), nil
}

func ValidateToolArgumentsAgainstSchema(args map[string]interface{}, schema map[string]interface{}) error {
	if args == nil {
		args = map[string]interface{}{}
	}
	if err := validateSchemaValue("", args, schema); err != nil {
		return err
	}
	return nil
}

func validateSchemaValue(path string, value interface{}, schema map[string]interface{}) error {
	if len(schema) == 0 {
		return nil
	}
	typeName := strings.ToLower(strings.TrimSpace(stringValue(schema["type"])))
	if typeName == "" && len(mapValue(schema["properties"])) > 0 {
		typeName = "object"
	}
	switch typeName {
	case "", "object":
		obj, ok := value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s must be an object", displayPath(path))
		}
		return validateSchemaObject(path, obj, schema)
	case "array":
		switch arr := value.(type) {
		case []interface{}:
			itemSchema := mapValue(schema["items"])
			for i, item := range arr {
				if err := validateSchemaValue(indexedPath(path, i), item, itemSchema); err != nil {
					return err
				}
			}
			return nil
		case []string:
			itemSchema := mapValue(schema["items"])
			for i, item := range arr {
				if err := validateSchemaValue(indexedPath(path, i), item, itemSchema); err != nil {
					return err
				}
			}
			return nil
		default:
			return fmt.Errorf("%s must be an array", displayPath(path))
		}
	case "string":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", displayPath(path))
		}
		if enumVals := stringList(schema["enum"]); len(enumVals) > 0 {
			for _, one := range enumVals {
				if s == one {
					return nil
				}
			}
			return fmt.Errorf("%s must be one of: %s", displayPath(path), strings.Join(enumVals, ", "))
		}
		return nil
	case "integer":
		number, ok := numericValue(value)
		if !ok || math.Trunc(number) != number {
			return fmt.Errorf("%s must be an integer", displayPath(path))
		}
		if min, ok := numericValue(schema["minimum"]); ok && number < min {
			return fmt.Errorf("%s must be >= %s", displayPath(path), formatNumber(min))
		}
		return nil
	case "number":
		number, ok := numericValue(value)
		if !ok {
			return fmt.Errorf("%s must be a number", displayPath(path))
		}
		if min, ok := numericValue(schema["minimum"]); ok && number < min {
			return fmt.Errorf("%s must be >= %s", displayPath(path), formatNumber(min))
		}
		return nil
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", displayPath(path))
		}
		return nil
	default:
		return nil
	}
}

func validateSchemaObject(path string, obj map[string]interface{}, schema map[string]interface{}) error {
	props := mapValue(schema["properties"])
	required := stringList(schema["required"])
	for _, key := range required {
		if _, ok := obj[key]; !ok {
			return fmt.Errorf("%s is required", displayPath(joinPath(path, key)))
		}
	}
	if additional, ok := boolValue(schema["additionalProperties"]); ok && !additional {
		for key := range obj {
			if _, exists := props[key]; !exists {
				return fmt.Errorf("%s is not allowed", displayPath(joinPath(path, key)))
			}
		}
	}
	for key, value := range obj {
		childSchema := mapValue(props[key])
		if len(childSchema) == 0 {
			continue
		}
		if err := validateSchemaValue(joinPath(path, key), value, childSchema); err != nil {
			return err
		}
	}
	return nil
}

func joinPath(prefix, segment string) string {
	segment = strings.TrimSpace(segment)
	if prefix == "" {
		return segment
	}
	if segment == "" {
		return prefix
	}
	return prefix + "." + segment
}

func indexedPath(prefix string, idx int) string {
	if prefix == "" {
		return fmt.Sprintf("[%d]", idx)
	}
	return fmt.Sprintf("%s[%d]", prefix, idx)
}

func displayPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "arguments"
	}
	return path
}

func mapValue(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func boolValue(v interface{}) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

func stringValue(v interface{}) string {
	s, _ := v.(string)
	return s
}

func stringList(v interface{}) []string {
	switch vv := v.(type) {
	case []string:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			s, ok := item.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func numericValue(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func formatNumber(v float64) string {
	if math.Trunc(v) == v {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%g", v)
}
