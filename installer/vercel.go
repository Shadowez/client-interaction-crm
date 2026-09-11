package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
)

type deploymentMetadata struct {
	ID      string
	Ready   bool
	Failed  bool
	Aliases []string
}

type vercelDeployResult struct {
	ID    string
	URL   string
	Ready bool
}

func parseVercelDeployResult(output string) (vercelDeployResult, error) {
	// Agent-mode Vercel output may contain human-readable progress followed by
	// a JSON result. Decode only an object with the explicit deployment fields.
	for offset := 0; offset < len(output); {
		relative := strings.IndexByte(output[offset:], '{')
		if relative < 0 {
			break
		}
		start := offset + relative
		var payload struct {
			Deployment struct {
				ID         string `json:"id"`
				URL        string `json:"url"`
				ReadyState string `json:"readyState"`
			} `json:"deployment"`
		}
		decoder := json.NewDecoder(strings.NewReader(output[start:]))
		if err := decoder.Decode(&payload); err == nil && payload.Deployment.URL != "" {
			url := normalizeWebURL(payload.Deployment.URL)
			if url == "" {
				return vercelDeployResult{}, fmt.Errorf("Vercel returned an invalid deployment URL")
			}
			return vercelDeployResult{ID: payload.Deployment.ID, URL: url, Ready: strings.EqualFold(payload.Deployment.ReadyState, "READY")}, nil
		}
		offset = start + 1
	}
	if url := lastHTTPSURL(output); url != "" {
		return vercelDeployResult{URL: url}, nil
	}
	return vercelDeployResult{}, fmt.Errorf("Vercel deployment completed without a production URL")
}

func deploymentInspectArgs(deploymentURL string) []string {
	return []string{"inspect", deploymentURL, "--wait", "--timeout", "10m", "--json"}
}

func (a *app) runVercel(ctx context.Context, args []string, options commandOptions) (string, error) {
	if a.vercelRun != nil {
		return a.vercelRun(ctx, args, options)
	}
	all := append([]string{"--yes", "vercel@" + vercelCLI}, args...)
	options.env = append(options.env, vercelEnvironment(filepath.Dir(a.node), options.interactive)...)
	return a.command(ctx, a.npx, all, options)
}

func vercelEnvironment(nodeBin string, interactive bool) []string {
	env := []string{nodePathEnvironment(nodeBin), "NO_UPDATE_NOTIFIER=1"}
	if !interactive {
		// @vercel/detect-agent documents AI_AGENT as its supported agent marker.
		// A distinct, unsupported agent name prevents Vercel from targeting a
		// local Claude Code installation for plugin install/update prompts.
		env = append(env, "AI_AGENT=client-interaction-crm-installer")
	}
	return env
}

func parseDeploymentMetadata(output string) (deploymentMetadata, error) {
	start := strings.IndexAny(output, "[{")
	if start < 0 {
		return deploymentMetadata{}, fmt.Errorf("Vercel returned no deployment JSON")
	}
	var value any
	if err := json.Unmarshal([]byte(output[start:]), &value); err != nil {
		return deploymentMetadata{}, fmt.Errorf("parse Vercel deployment status: %w", err)
	}
	metadata := deploymentMetadata{}
	if root, ok := value.(map[string]any); ok {
		for _, key := range []string{"id", "uid"} {
			if id, ok := root[key].(string); ok && id != "" {
				metadata.ID = id
				break
			}
		}
	}
	walkDeploymentMetadata(value, "", &metadata)
	metadata.Aliases = uniqueURLs(metadata.Aliases)
	return metadata, nil
}

func walkDeploymentMetadata(value any, key string, metadata *deploymentMetadata) {
	switch item := value.(type) {
	case []any:
		for _, child := range item {
			walkDeploymentMetadata(child, key, metadata)
		}
	case map[string]any:
		for childKey, child := range item {
			walkDeploymentMetadata(child, strings.ToLower(childKey), metadata)
		}
	case string:
		upper := strings.ToUpper(item)
		if key == "state" || key == "status" || key == "readystate" {
			if upper == "READY" {
				metadata.Ready = true
			}
			if upper == "ERROR" || upper == "FAILED" || upper == "CANCELED" || upper == "CANCELLED" {
				metadata.Failed = true
			}
		}
		if key == "alias" || key == "aliases" {
			if normalized := normalizeWebURL(item); normalized != "" {
				metadata.Aliases = append(metadata.Aliases, normalized)
			}
		}
	}
}

func normalizeWebURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return ""
	}
	parsed.Path, parsed.RawPath, parsed.RawQuery, parsed.Fragment = "", "", "", ""
	return strings.TrimRight(parsed.String(), "/")
}

func uniqueURLs(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func selectCanonicalURL(deploymentURL string, aliases []string) string {
	deployment := normalizeWebURL(deploymentURL)
	deploymentHost := hostOf(deployment)
	candidates := []string{}
	for _, alias := range aliases {
		normalized := normalizeWebURL(alias)
		if normalized != "" && hostOf(normalized) != deploymentHost {
			candidates = append(candidates, normalized)
		}
	}
	candidates = uniqueURLs(candidates)
	sort.SliceStable(candidates, func(i, j int) bool {
		iVercel := strings.HasSuffix(hostOf(candidates[i]), ".vercel.app")
		jVercel := strings.HasSuffix(hostOf(candidates[j]), ".vercel.app")
		if iVercel != jVercel {
			return !iVercel
		}
		if len(candidates[i]) != len(candidates[j]) {
			return len(candidates[i]) < len(candidates[j])
		}
		return candidates[i] < candidates[j]
	})
	if len(candidates) > 0 {
		return candidates[0]
	}
	return deployment
}

func hostOf(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
