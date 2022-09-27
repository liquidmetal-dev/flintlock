package cloudhypervisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/carlmjohnson/requests"
)

// CustomErrValidator is a custom response handler that will set a custom error message based on the status code.
func CustomErrValidator(failureMapping map[int]string) requests.ResponseHandler {
	return func(res *http.Response) error {
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			return nil
		}

		for errorCode, errorMessage := range failureMapping {
			if errorCode == res.StatusCode {
				return errors.New(errorMessage)
			}
		}

		if res.ContentLength > 0 {
			data, _ := io.ReadAll(res.Body)
			return fmt.Errorf("%w: unexpected status: %d: %s",
				(*requests.ResponseError)(res), res.StatusCode, string(data))
		}

		return fmt.Errorf("%w: unexpected status: %d",
			(*requests.ResponseError)(res), res.StatusCode)
	}
}

// ToJSONForCode is a custom response handler that will unmarshal the http response body into the specific struct
// if the status code matches. Otherwise the destination is set to nil.
func ToJSONForCode(code int, dest interface{}) requests.ResponseHandler {
	return func(res *http.Response) error {
		if code != res.StatusCode {
			dest = nil
			return nil
		}

		data, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(data, dest); err != nil {
			return err
		}
		return nil
	}
}
