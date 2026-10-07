package audit

import (
	"regexp"
	"strings"
)

var (
	reAWSAccessKey  = regexp.MustCompile(`\b(AKIA|ABIA|ACCA|ASIA)[0-9A-Z]{16}\b`)
	reGitHubToken   = regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{36,255}\b`)
	reGitHubFine    = regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{82}\b`)
	reOpenAIAnth    = regexp.MustCompile(`\bsk-[A-Za-z0-9_\-]{20,}\b`)
	reSlackToken    = regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z_\-]{10,}\b`)
	reStripeKey     = regexp.MustCompile(`\b(sk|pk|rk)_(live|test)_[0-9a-zA-Z]{24,}\b`)
	rePrivateKey    = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)
	reBearer        = regexp.MustCompile(`(?i)\b(bearer)\s+([A-Za-z0-9_\-\.]{20,})\b`)
	reAuthHeader    = regexp.MustCompile(`(?i)\b(authorization)\s*[:=]\s*['"]?([^'"\s]{20,})['"]?`)
	reEnvLine       = regexp.MustCompile(`(?m)^([ \t]*[A-Za-z0-9_]*(?:KEY|SECRET|TOKEN|PASSWORD|PASS|CREDENTIAL)[A-Za-z0-9_]*\s*=\s*).*$`)
	reAssignmentKey = regexp.MustCompile(`(?i)\b([A-Za-z0-9_]*(?:PASSWORD|PASSWD|SECRET|API_KEY|APIKEY|TOKEN|AUTH_TOKEN|PRIVATE_KEY)[A-Za-z0-9_]*)\s*([:=])\s*['"]?([^'"\s&;]{4,})['"]?`)
)

func RedactText(s string) string {
	if s == "" {
		return ""
	}

	// 1. Multiline private keys
	s = rePrivateKey.ReplaceAllString(s, "[REDACTED_PRIVATE_KEY]")

	// 2. Specific API keys and token formats (replace specific signatures first)
	s = reAWSAccessKey.ReplaceAllString(s, "[REDACTED_AWS_KEY]")
	s = reGitHubToken.ReplaceAllString(s, "[REDACTED_GITHUB_TOKEN]")
	s = reGitHubFine.ReplaceAllString(s, "[REDACTED_GITHUB_TOKEN]")
	s = reOpenAIAnth.ReplaceAllString(s, "[REDACTED_API_KEY]")
	s = reSlackToken.ReplaceAllString(s, "[REDACTED_SLACK_TOKEN]")
	s = reStripeKey.ReplaceAllString(s, "[REDACTED_STRIPE_KEY]")

	// 3. Authorization Bearer header
	s = reBearer.ReplaceAllString(s, "$1 [REDACTED_AUTH_TOKEN]")
	s = reAuthHeader.ReplaceAllString(s, "$1: [REDACTED_AUTH_HEADER]")

	// 4. Dotenv-style full lines with sensitive variable names (unless already redacted)
	s = reEnvLine.ReplaceAllStringFunc(s, func(match string) string {
		if strings.Contains(match, "[REDACTED") {
			return match
		}
		sub := reEnvLine.FindStringSubmatch(match)
		if len(sub) > 1 {
			return sub[1] + "[REDACTED_ENV_VALUE]"
		}
		return match
	})

	// 5. Generic key-value secret assignments in scripts / CLI invocations (unless already redacted)
	s = reAssignmentKey.ReplaceAllStringFunc(s, func(match string) string {
		if strings.Contains(match, "[REDACTED") {
			return match
		}
		sub := reAssignmentKey.FindStringSubmatch(match)
		if len(sub) > 2 {
			return sub[1] + sub[2] + "[REDACTED_SECRET]"
		}
		return match
	})

	return s
}

func RedactStringSlice(slice []string) []string {
	if len(slice) == 0 {
		return slice
	}
	out := make([]string, len(slice))
	for i, s := range slice {
		out[i] = RedactText(s)
	}
	return out
}

func RedactMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		kLower := strings.ToLower(k)
		if strings.Contains(kLower, "key") ||
			strings.Contains(kLower, "secret") ||
			strings.Contains(kLower, "token") ||
			strings.Contains(kLower, "password") ||
			strings.Contains(kLower, "auth") ||
			strings.Contains(kLower, "credential") {
			out[k] = "[REDACTED_FIELD]"
			continue
		}

		switch val := v.(type) {
		case string:
			out[k] = RedactText(val)
		case map[string]interface{}:
			out[k] = RedactMap(val)
		case []interface{}:
			out[k] = redactSlice(val)
		default:
			out[k] = val
		}
	}
	return out
}

func redactSlice(slice []interface{}) []interface{} {
	out := make([]interface{}, len(slice))
	for i, v := range slice {
		switch val := v.(type) {
		case string:
			out[i] = RedactText(val)
		case map[string]interface{}:
			out[i] = RedactMap(val)
		case []interface{}:
			out[i] = redactSlice(val)
		default:
			out[i] = val
		}
	}
	return out
}
