package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const (
	antigravityLoadCodeAssistPath = "/v1internal:loadCodeAssist"
	antigravityFetchModelsPath    = "/v1internal:fetchAvailableModels"
	quotaCacheTTL                 = 5 * time.Minute
)

// quotaCacheEntry holds cached quota data with expiration
type quotaCacheEntry struct {
	data      *cliproxyauth.AntigravityQuotaData
	expiresAt time.Time
}

var (
	quotaCache   = make(map[string]*quotaCacheEntry)
	quotaCacheMu sync.RWMutex
)

// FetchAntigravityQuota retrieves quota information for an Antigravity account
// It uses a 5-minute cache to avoid excessive API calls
func FetchAntigravityQuota(ctx context.Context, auth *cliproxyauth.Auth, cfg *config.Config) (*cliproxyauth.AntigravityQuotaData, error) {
	if auth == nil {
		return nil, fmt.Errorf("auth is nil")
	}

	// Check cache first
	quotaCacheMu.RLock()
	if entry, exists := quotaCache[auth.ID]; exists && time.Now().Before(entry.expiresAt) {
		quotaCacheMu.RUnlock()
		log.Debugf("antigravity quota: using cached data for account %s", auth.ID)
		return entry.data, nil
	}
	quotaCacheMu.RUnlock()

	// Get access token
	executor := &AntigravityExecutor{cfg: cfg}
	token, updatedAuth, err := executor.ensureAccessToken(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}
	if updatedAuth != nil {
		auth = updatedAuth
	}
	if token == "" {
		return nil, fmt.Errorf("access token is empty")
	}

	// Create HTTP client
	httpClient := newProxyAwareHTTPClient(ctx, cfg, auth, 0)

	// Get base URL
	baseURLs := antigravityBaseURLFallbackOrder(auth)
	if len(baseURLs) == 0 {
		return nil, fmt.Errorf("no base URLs available")
	}
	baseURL := baseURLs[0]

	// Fetch project ID and subscription tier
	projectID, tier, err := fetchProjectID(ctx, token, httpClient, baseURL)
	if err != nil {
		// Check if it's a 403 Forbidden error
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "forbidden") {
			log.Warnf("antigravity quota: account %s lacks quota access permission", auth.ID)
			result := &cliproxyauth.AntigravityQuotaData{
				IsForbidden: true,
				FetchedAt:   time.Now(),
			}
			// Cache the forbidden result
			quotaCacheMu.Lock()
			quotaCache[auth.ID] = &quotaCacheEntry{
				data:      result,
				expiresAt: time.Now().Add(quotaCacheTTL),
			}
			quotaCacheMu.Unlock()
			return result, nil
		}
		return nil, fmt.Errorf("failed to fetch project ID: %w", err)
	}

	// Fetch quota data
	models, err := fetchQuotaData(ctx, token, projectID, httpClient, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quota data: %w", err)
	}

	result := &cliproxyauth.AntigravityQuotaData{
		Models:           models,
		SubscriptionTier: tier,
		ProjectID:        projectID,
		FetchedAt:        time.Now(),
		IsForbidden:      false,
	}

	// Update cache
	quotaCacheMu.Lock()
	quotaCache[auth.ID] = &quotaCacheEntry{
		data:      result,
		expiresAt: time.Now().Add(quotaCacheTTL),
	}
	quotaCacheMu.Unlock()

	log.Debugf("antigravity quota: fetched fresh data for account %s (tier: %s, models: %d)",
		auth.ID, tier, len(models))

	return result, nil
}

// fetchProjectID retrieves the project ID and subscription tier from loadCodeAssist
func fetchProjectID(ctx context.Context, token string, httpClient *http.Client, baseURL string) (string, string, error) {
	url := strings.TrimSuffix(baseURL, "/") + antigravityLoadCodeAssistPath

	reqBody := []byte(`{}`)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("User-Agent", defaultAntigravityAgent)
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return "", "", fmt.Errorf("API returned status %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	// Parse response
	projectID := gjson.GetBytes(bodyBytes, "projectId").String()
	tier := gjson.GetBytes(bodyBytes, "subscriptionTier").String()

	if projectID == "" {
		return "", "", fmt.Errorf("project_id not found in response")
	}

	// Default tier to FREE if not specified
	if tier == "" {
		tier = "FREE"
	}

	return projectID, tier, nil
}

// fetchQuotaData retrieves quota information for all models
func fetchQuotaData(ctx context.Context, token, projectID string, httpClient *http.Client, baseURL string) (map[string]*cliproxyauth.AntigravityModelQuota, error) {
	url := strings.TrimSuffix(baseURL, "/") + antigravityFetchModelsPath

	reqBody := []byte(`{}`)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("User-Agent", defaultAntigravityAgent)
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned status %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	// Parse models
	modelsResult := gjson.GetBytes(bodyBytes, "models")
	if !modelsResult.Exists() {
		return nil, fmt.Errorf("models field not found in response")
	}

	quotaMap := make(map[string]*cliproxyauth.AntigravityModelQuota)

	modelsResult.ForEach(func(modelName, modelData gjson.Result) bool {
		quotaInfo := modelData.Get("quotaInfo")
		if !quotaInfo.Exists() {
			return true // Skip models without quota info
		}

		remainingFraction := quotaInfo.Get("remainingFraction").Float()
		resetTimeStr := quotaInfo.Get("resetTime").String()

		var resetTime time.Time
		if resetTimeStr != "" {
			parsed, err := time.Parse(time.RFC3339, resetTimeStr)
			if err == nil {
				resetTime = parsed
			}
		}

		quotaMap[modelName.String()] = &cliproxyauth.AntigravityModelQuota{
			RemainingFraction: remainingFraction,
			ResetTime:         resetTime,
		}

		return true
	})

	return quotaMap, nil
}

// ClearQuotaCache removes cached quota data for a specific account or all accounts
func ClearQuotaCache(accountID string) {
	quotaCacheMu.Lock()
	defer quotaCacheMu.Unlock()

	if accountID == "" {
		// Clear all cache
		quotaCache = make(map[string]*quotaCacheEntry)
		log.Debug("antigravity quota: cleared all cache")
	} else {
		// Clear specific account
		delete(quotaCache, accountID)
		log.Debugf("antigravity quota: cleared cache for account %s", accountID)
	}
}

// CleanExpiredQuotaCache removes expired entries from the cache
func CleanExpiredQuotaCache() {
	quotaCacheMu.Lock()
	defer quotaCacheMu.Unlock()

	now := time.Now()
	for id, entry := range quotaCache {
		if now.After(entry.expiresAt) {
			delete(quotaCache, id)
		}
	}
}
