package eventsourcing

import "encoding/json"

type EncoderJSON struct{}

func (e EncoderJSON) Serialize(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (e EncoderJSON) Deserialize(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
