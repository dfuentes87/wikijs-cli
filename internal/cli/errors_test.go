package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/dfuentes87/wikijs-cli/internal/api"
	"github.com/dfuentes87/wikijs-cli/internal/config"
)

func TestFormatError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "missing config",
			err:  errors.Join(config.ErrMissing, errors.New("/tmp/wikijs.json")),
			want: "Config error:",
		},
		{
			name: "invalid config",
			err:  errors.Join(config.ErrInvalid, errors.New(`missing "url"`)),
			want: "Config error:",
		},
		{
			name: "auth",
			err:  api.AuthError{Status: "401 Unauthorized"},
			want: "Authentication failed:",
		},
		{
			name: "not found",
			err:  errors.Join(api.ErrNotFound, errors.New("page /missing")),
			want: "Page not found:",
		},
		{
			name: "graphql",
			err:  api.GraphQLErrors{{Message: "field not found"}},
			want: "GraphQL error: field not found",
		},
		{
			name: "generic",
			err:  errors.New("boom"),
			want: "Error: boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatError(tt.err)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("FormatError() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestFormatErrorTreatsForbiddenGraphQLErrorsAsAuth(t *testing.T) {
	err := api.GraphQLErrors{{Message: "Forbidden"}, {Message: "Forbidden"}}
	got := FormatError(err)
	if !strings.Contains(got, "Authentication failed") {
		t.Fatalf("FormatError() = %q", got)
	}
}

func TestPrintErrorColor(t *testing.T) {
	var buf bytes.Buffer
	PrintErrorColor(&buf, errors.New("boom"), true)
	if !strings.Contains(buf.String(), "\033[") {
		t.Fatalf("colored error missing ANSI: %q", buf.String())
	}
	buf.Reset()
	PrintErrorColor(&buf, errors.New("boom"), false)
	if strings.Contains(buf.String(), "\033[") {
		t.Fatalf("plain error contains ANSI: %q", buf.String())
	}
}
