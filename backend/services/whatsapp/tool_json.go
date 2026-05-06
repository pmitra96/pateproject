package whatsapp

import "encoding/json"

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

