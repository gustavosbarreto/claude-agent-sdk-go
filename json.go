package claude

import "encoding/json"

// marshalWithType adds a "type" field to the JSON output of a struct.
func marshalWithType(typ string, v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// Insert "type":"<typ>" at the beginning.
	if len(data) < 2 || data[0] != '{' {
		return data, nil
	}
	typeField := `"type":"` + typ + `",`
	out := make([]byte, 0, len(data)+len(typeField))
	out = append(out, '{')
	out = append(out, typeField...)
	out = append(out, data[1:]...)
	return out, nil
}
