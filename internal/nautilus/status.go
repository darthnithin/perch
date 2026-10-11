package nautilus

import "encoding/json"

// Some transports deliver a JSON Status instead of a transport error. Classify
// only its stable code; never surface the untrusted message or details.
func apiStatusError(data []byte) *QueryError {
	var status struct {
		Kind, APIVersion string
		Code             int
	}
	if json.Unmarshal(data, &status) != nil || status.Kind != "Status" || status.APIVersion != "v1" {
		return nil
	}
	if validateJSON(data) != nil {
		return queryError(InvalidResponse, "Invalid namespace API status")
	}
	return httpStatusError(status.Code)
}

func httpStatusError(code int) *QueryError {
	kind := ReadFailed
	switch code {
	case 401:
		kind = Unauthenticated
	case 403:
		kind = Forbidden
	case 404:
		kind = NotFound
	case 410:
		kind = Expired
	case 301, 302, 303, 307, 308:
		return queryError(InvalidResponse, "Namespace API redirected the request; check the connection endpoint")
	case 429, 500, 502, 503, 504:
		kind = Unavailable
	}
	return readError(queryError(kind, ""))
}
