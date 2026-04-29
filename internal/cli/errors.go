package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dfuentes87/wikijs-cli/internal/api"
	"github.com/dfuentes87/wikijs-cli/internal/config"
	"github.com/dfuentes87/wikijs-cli/internal/output"
)

func PrintError(w io.Writer, err error) {
	fmt.Fprintln(w, FormatError(err))
}

func PrintErrorColor(w io.Writer, err error, colorEnabled bool) {
	fmt.Fprintln(w, output.Color(colorEnabled, output.Red, FormatError(err)))
}

func FormatError(err error) string {
	if err == nil {
		return ""
	}

	var authErr api.AuthError
	if errors.As(err, &authErr) || errors.Is(err, api.ErrAuth) {
		if authErr.Status != "" {
			return "Authentication failed: Wiki.js rejected the API token or permissions (" + authErr.Status + ")."
		}
		return "Authentication failed: Wiki.js rejected the API token or permissions."
	}

	if errors.Is(err, config.ErrMissing) {
		return "Config error: " + err.Error()
	}
	if errors.Is(err, config.ErrInvalid) {
		return "Config error: " + trimSentinel(err.Error(), config.ErrInvalid.Error()+": ")
	}
	if errors.Is(err, api.ErrNotFound) {
		return "Page not found: " + trimSentinel(err.Error(), api.ErrNotFound.Error()+": ")
	}

	var gqlErrs api.GraphQLErrors
	if errors.As(err, &gqlErrs) {
		if graphQLErrorsAreForbidden(gqlErrs) {
			return "Authentication failed: Wiki.js rejected the API token or permissions."
		}
		return "GraphQL error: " + gqlErrs.Error()
	}

	return "Error: " + err.Error()
}

func graphQLErrorsAreForbidden(errs api.GraphQLErrors) bool {
	if len(errs) == 0 {
		return false
	}
	for _, err := range errs {
		if !strings.EqualFold(strings.TrimSpace(err.Message), "forbidden") {
			return false
		}
	}
	return true
}

func trimSentinel(message, prefix string) string {
	return strings.TrimPrefix(message, prefix)
}
