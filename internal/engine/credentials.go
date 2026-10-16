package engine

import (
	"regexp"
	"strings"

	"github.com/cemilalp02/portcullis/internal/ruleset"
)

// A token in an Authorization header is how APIs are used; the same token
// sent to a host it does not belong to is how it is stolen. credentialHosts
// lists well-known credential variables and the domains they are issued for.
var credentialHosts = []struct {
	vars  *regexp.Regexp
	hosts []string
}{
	{regexp.MustCompile(`\b(github_token|gh_token|github_pat|gh_pat|gh_enterprise_token)\b`), []string{"github.com", "githubusercontent.com", "ghcr.io", "github.io"}},
	{regexp.MustCompile(`\b(aws_secret_access_key|aws_session_token)\b`), []string{"amazonaws.com", "aws.amazon.com", "amazonaws.com.cn"}},
	{regexp.MustCompile(`\b(openai_api_key)\b`), []string{"openai.com", "openai.azure.com", "azure.com"}},
	{regexp.MustCompile(`\b(anthropic_api_key|anthropic_auth_token)\b`), []string{"anthropic.com", "claude.ai"}},
	{regexp.MustCompile(`\b(npm_token|npm_auth_token|node_auth_token)\b`), []string{"npmjs.org", "npmjs.com", "github.com"}},
	{regexp.MustCompile(`\b(hf_token|huggingface_token|hugging_face_hub_token|huggingface_hub_token)\b`), []string{"huggingface.co", "hf.co"}},
	{regexp.MustCompile(`\b(gitlab_token|gl_token|gitlab_private_token|ci_job_token)\b`), []string{"gitlab.com"}},
	{regexp.MustCompile(`\b(slack_bot_token|slack_user_token)\b`), []string{"slack.com"}},
	{regexp.MustCompile(`\b(stripe_secret_key|stripe_api_key)\b`), []string{"stripe.com"}},
	{regexp.MustCompile(`\b(gemini_api_key|google_api_key)\b`), []string{"googleapis.com", "google.com"}},
}

var (
	// a network call on the line: command-line tools and client libraries
	sendsRE = regexp.MustCompile(`\b(curl|wget|iwr|irm|invoke-webrequest|invoke-restmethod|httpie|xh)\b|\bhttps?\s+(get|post|put)\b|\b(requests|httpx)\.(get|post|put|patch|request)\(|\burlopen\(|\bfetch\(|\baxios(\.[a-z]+)?\(|\bhttp\.request\(`)
	hostRE  = regexp.MustCompile(`https?://([a-z0-9.-]+)`)
)

// credentialDestination reports a line that sends a well-known credential to
// a URL outside the domains it is issued for. Lines whose destination is not
// written out (a variable, localhost) are left alone: the check only speaks
// when it can tell.
func credentialDestination(c *collector, r *ruleset.Compiled) {
	view := c.normalized()
	text := view.Text
	for base := 0; base < len(text); {
		end := strings.IndexByte(text[base:], '\n')
		if end < 0 {
			end = len(text) - base
		}
		line := text[base : base+end]
		if loc := sendsRE.FindStringIndex(line); loc != nil {
			for _, ch := range credentialHosts {
				v := ch.vars.FindStringIndex(line)
				if v == nil {
					continue
				}
				if host, ok := foreignHost(line, ch.hosts); ok {
					start, stop := view.Raw(base+v[0]), view.Raw(base+v[1])
					c.add(r, start, stop, c.doc.Text[start:stop], map[string]string{
						"name": line[v[0]:v[1]], "match": host,
					})
				}
			}
		}
		base += end + 1
	}
}

// foreignHost returns the first host on the line that is not one of allowed
// (or a subdomain), ignoring local addresses.
func foreignHost(line string, allowed []string) (string, bool) {
	for _, m := range hostRE.FindAllStringSubmatch(line, -1) {
		host := strings.TrimSuffix(m[1], ".")
		if host == "localhost" || strings.HasPrefix(host, "127.") || host == "0.0.0.0" || strings.HasSuffix(host, ".localhost") {
			continue
		}
		ok := false
		for _, a := range allowed {
			if host == a || strings.HasSuffix(host, "."+a) {
				ok = true
				break
			}
		}
		if !ok {
			return host, true
		}
	}
	return "", false
}
