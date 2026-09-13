package anna

import (
	"errors"
	"net/url"
)

// API request URLs contain the account key; file URLs contain signed tokens.
func withoutRequestURL(err error) error {
	var requestError *url.Error
	if errors.As(err, &requestError) {
		return requestError.Err
	}
	return err
}
