package cloudhypervisor

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/carlmjohnson/requests"
)

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
