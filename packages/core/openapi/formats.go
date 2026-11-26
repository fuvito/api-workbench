package openapi

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// SupportedFormats lists the string formats whose values are checked. Other formats are accepted
// without a check, as the OpenAPI specification allows.
var SupportedFormats = []string{"date", "date-time", "email", "uuid", "ipv4", "ipv6", "byte", "uri", "hostname"}

// The validation library keeps its format validators in package-level maps, so they apply to every
// check in the process: responses, generated requests and the schema assertion.
func init() {
	openapi3.DefineStringFormatValidator("email", openapi3.NewCallbackValidator(validateEmail))
	openapi3.DefineStringFormatValidator("uuid", openapi3.NewRegexpFormatValidator(openapi3.FormatOfStringForUUIDOfRFC9562))
	openapi3.DefineStringFormatValidator("uri", openapi3.NewCallbackValidator(validateURI))
	openapi3.DefineStringFormatValidator("hostname", openapi3.NewCallbackValidator(validateHostname))
	openapi3.DefineIPv4Format()
	openapi3.DefineIPv6Format()
}

// validateEmail accepts a single address, with no display name, comments or spaces. It is looser
// than the full grammar on purpose (a domain without a dot, such as user@localhost, is allowed) so
// a correct server is never reported for a rare but valid address.
func validateEmail(v string) error {
	if v == "" || strings.ContainsAny(v, " \t\r\n<>,;") {
		return fmt.Errorf("not an email address")
	}
	addr, err := mail.ParseAddress(v)
	if err != nil || addr.Address != v || addr.Name != "" {
		return fmt.Errorf("not an email address")
	}
	return nil
}

var (
	reURIScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
	// a % that is not followed by two hex digits
	reBadEscape = regexp.MustCompile(`%($|[^0-9A-Fa-f]|[0-9A-Fa-f]($|[^0-9A-Fa-f]))`)
	reHostLabel = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
)

// validateURI accepts a URI as RFC 3986 means it: with a scheme. A relative link is a
// "uri-reference", which is not checked. Letters outside ASCII are let through, although a strict
// URI would write them as escapes: servers send them, and they break few clients.
func validateURI(v string) error {
	bad := func() error { return fmt.Errorf("not a URI with a scheme, such as https://") }
	if !reURIScheme.MatchString(v) || reBadEscape.MatchString(v) {
		return bad()
	}
	for _, r := range v {
		if r <= ' ' || r == 0x7f || strings.ContainsRune("<>\"{}|\\^`", r) {
			return bad()
		}
	}
	if _, err := url.Parse(v); err != nil {
		return bad()
	}
	return nil
}

// validateHostname accepts a host name as RFC 1123 means it: labels of letters, digits and inner
// hyphens, at most 63 each and 253 together, with an optional dot at the end. A name written in
// another alphabet is an "idn-hostname", which is not checked.
func validateHostname(v string) error {
	name := strings.TrimSuffix(v, ".")
	if name == "" || len(name) > 253 {
		return fmt.Errorf("not a host name")
	}
	for _, label := range strings.Split(name, ".") {
		if !reHostLabel.MatchString(label) {
			return fmt.Errorf("not a host name")
		}
	}
	return nil
}

var reNewFormatExample = regexp.MustCompile(`doesn't match the format "(email|uuid|ipv4|ipv6|uri|hostname)"`)

// onlyAFormatExample reports whether a spec validation error is an example that breaks one of the
// formats this package registers.
func onlyAFormatExample(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "invalid example") && reNewFormatExample.MatchString(msg)
}
