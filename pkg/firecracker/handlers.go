package firecracker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/carlmjohnson/requests"
)

func faultValidator(res *http.Response) error {
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}

	respErr := (*requests.ResponseError)(res)

	data, _ := io.ReadAll(res.Body)

	fault := Fault{}
	if err := json.Unmarshal(data, &fault); err == nil && fault.FaultMessage != "" {
		return fmt.Errorf("%w: %s", respErr, fault.FaultMessage)
	}

	if len(data) > 0 {
		return fmt.Errorf("%w: %s", respErr, string(data))
	}

	return respErr
}
