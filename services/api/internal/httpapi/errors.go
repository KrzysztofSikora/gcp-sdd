package httpapi

import "net/http"

// apiError is a JSON error response: {"error": {"code", "message"}} (specs/design.md).
type apiError struct {
	status  int
	code    string
	message string
}

var (
	errInvalidJSON          = &apiError{http.StatusBadRequest, "invalid_json", `Body must be one JSON object with only the field "text".`}
	errInvalidText          = &apiError{http.StatusBadRequest, "invalid_text", `Field "text" is required and must be a string.`}
	errEmptyText            = &apiError{http.StatusBadRequest, "empty_text", `Field "text" must not be empty or whitespace only.`}
	errTextTooLong          = &apiError{http.StatusBadRequest, "text_too_long", `Field "text" must be at most 10000 characters.`}
	errNotFound             = &apiError{http.StatusNotFound, "not_found", "No such endpoint."}
	errMethodNotAllowed     = &apiError{http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed for this endpoint."}
	errPayloadTooLarge      = &apiError{http.StatusRequestEntityTooLarge, "payload_too_large", "Request body must be at most 256 KiB."}
	errUnsupportedMediaType = &apiError{http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json."}
	errInternal             = &apiError{http.StatusInternalServerError, "internal_error", "Internal server error."}
	errAnalyzerUnavailable  = &apiError{http.StatusBadGateway, "analyzer_unavailable", "The analysis service is unavailable."}
	errAnalyzerError        = &apiError{http.StatusBadGateway, "analyzer_error", "The analysis service returned an invalid response."}
	errAnalyzerTimeout      = &apiError{http.StatusGatewayTimeout, "analyzer_timeout", "The analysis service did not respond in time."}
)

func writeError(w http.ResponseWriter, e *apiError) {
	type body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	writeJSON(w, e.status, map[string]body{"error": {Code: e.code, Message: e.message}})
}
