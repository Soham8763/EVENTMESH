package events

import "encoding/json"

func Serialize(event interface{}) ([]byte, error) {
	return json.Marshal(event)
}

func Deserialize(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
