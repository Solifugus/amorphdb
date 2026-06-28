package types

import (
	"encoding/json"
	"fmt"
)

// SerializeValue serializes any value to bytes for storage or hashing
func SerializeValue(value interface{}) []byte {
	switch v := value.(type) {
	case Text:
		return v.Serialize()
	case Number:
		return v.Serialize()
	case Time:
		return v.Serialize()
	case Money:
		return v.Serialize()
	case Picture:
		return v.Serialize()
	case Reference:
		return v.Serialize()
	case Nothing:
		return v.Serialize()
	case Unknown:
		return v.Serialize()
	case Anything:
		return v.Serialize()
	case Boolean:
		return v.Serialize()
	case List:
		return v.Serialize()
	case Record:
		return v.Serialize()
	case string:
		return Text{Value: v}.Serialize()
	case float64:
		return Number{Value: v}.Serialize()
	case int:
		return Number{Value: float64(v)}.Serialize()
	case int64:
		return Number{Value: float64(v)}.Serialize()
	case int32:
		return Number{Value: float64(v)}.Serialize()
	case uint:
		return Number{Value: float64(v)}.Serialize()
	case uint32:
		return Number{Value: float64(v)}.Serialize()
	case uint64:
		// Agent/author IDs are uint64; represent as Number.
		return Number{Value: float64(v)}.Serialize()
	case bool:
		return Boolean{Value: v}.Serialize()
	case []interface{}:
		// Convert to List
		return List{Elements: v}.Serialize()
	case map[string]interface{}:
		// Convert to Record
		return Record{Fields: v}.Serialize()
	case nil:
		return Nothing{}.Serialize()
	default:
		// Fallback to JSON serialization for unknown types
		data, err := json.Marshal(v)
		if err != nil {
			return Unknown{Reason: fmt.Sprintf("serialization error: %v", err)}.Serialize()
		}
		return data
	}
}
