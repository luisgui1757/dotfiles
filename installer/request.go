package installer

import "errors"

// A machine request must distinguish an intentional empty selection from a
// missing field. Retry and maintenance modes derive choices from durable intent.
func (request *Request) UnmarshalJSON(data []byte) error {
	type wireRequest Request
	var decoded wireRequest
	if err := Decode(data, &decoded); err != nil {
		return err
	}
	if decoded.Mode == "apply" && !decoded.Retry && decoded.Selected == nil {
		return errors.New("apply requires an explicit selected array; use [] to request an empty selection")
	}
	*request = Request(decoded)
	return nil
}
