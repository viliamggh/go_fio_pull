package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

const (
	format             = "json"
	defaultHTTPTimeout = 90 * time.Second
)

var (
	debug                = os.Getenv("DEBUG") != ""
	keyVaultURL          = getEnvOrDefault("KEY_VAULT_URL", "https://kv-fintrack-dev.vault.azure.net")
	storageAccountURL    = getEnvOrDefault("STORAGE_ACCOUNT_URL", "https://safintrackdev.blob.core.windows.net/")
	storageContainerName = getEnvOrDefault("STORAGE_CONTAINER_NAME", "raw")
	accountAliases       = getEnvOrDefault("ACCOUNT_ALIASES", "invoices")
	httpClient           = newHTTPClient(getEnvDuration("HTTP_CLIENT_TIMEOUT", defaultHTTPTimeout))
)

// getEnvOrDefault retrieves an environment variable or returns a default value.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	log.Printf("Warning: %s not set, using default: %s", key, defaultValue)
	return defaultValue
}

// getEnvDuration retrieves an environment variable as duration or returns a default value.
// The format matches time.ParseDuration (e.g. "90s", "2m").
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
		log.Printf("Warning: %s has invalid duration %q, using default: %s", key, value, defaultValue)
	}
	return defaultValue
}

// getAccountAliases parses comma-separated account aliases from env var
func getAccountAliases() []string {
	aliases := strings.Split(accountAliases, ",")
	result := make([]string, 0, len(aliases))
	for _, a := range aliases {
		trimmed := strings.TrimSpace(a)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return []string{"invoices"} // Default fallback
	}
	return result
}

// AccountResult holds the result of processing a single account
type AccountResult struct {
	Account string
	Success bool
	Message string
	Error   error
}

func main() {
	// Configure the logger for local development.
	// In production you may want to redirect logs to a file or disable debug logs.
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/", handler)
	log.Println("Server is starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// healthHandler responds to health checks without doing any heavy operations
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handler fetches transaction data from FIO API for all configured accounts
// and writes each to Azure Blob Storage. Continues processing even if one account fails.
func handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request from %s", r.RemoteAddr)

	cred, err := azAuth()
	if err != nil {
		log.Printf("Authentication failed: %v", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	startDate, endDate := getDatesFromQuery(r)
	log.Printf("Fetching transactions from %s to %s", startDate, endDate)

	accounts := getAccountAliases()
	log.Printf("Processing %d accounts: %v", len(accounts), accounts)

	results := make([]AccountResult, 0, len(accounts))
	successCount := 0

	for _, account := range accounts {
		result := processAccount(cred, account, startDate, endDate)
		results = append(results, result)
		if result.Success {
			successCount++
		}
	}

	// Build response
	w.Header().Set("Content-Type", "application/json")

	if successCount == 0 {
		// All accounts failed
		w.WriteHeader(http.StatusInternalServerError)
	} else if successCount < len(accounts) {
		// Partial success
		w.WriteHeader(http.StatusPartialContent) // 206
	} else {
		// All succeeded
		w.WriteHeader(http.StatusOK)
	}

	// Write results summary
	fmt.Fprintf(w, "{\n")
	fmt.Fprintf(w, "  \"processed\": %d,\n", len(accounts))
	fmt.Fprintf(w, "  \"succeeded\": %d,\n", successCount)
	fmt.Fprintf(w, "  \"failed\": %d,\n", len(accounts)-successCount)
	fmt.Fprintf(w, "  \"results\": [\n")
	for i, r := range results {
		comma := ","
		if i == len(results)-1 {
			comma = ""
		}
		status := "success"
		errMsg := ""
		if !r.Success {
			status = "failed"
			if r.Error != nil {
				errMsg = r.Error.Error()
			}
		}
		fmt.Fprintf(w, "    {\"account\": \"%s\", \"status\": \"%s\", \"message\": \"%s\", \"error\": \"%s\"}%s\n",
			r.Account, status, r.Message, errMsg, comma)
	}
	fmt.Fprintf(w, "  ]\n")
	fmt.Fprintf(w, "}\n")
}

// processAccount handles fetching and storing data for a single account
func processAccount(cred *azidentity.DefaultAzureCredential, account, startDate, endDate string) AccountResult {
	log.Printf("[%s] Starting processing", account)

	// Token secret name follows pattern: fio-token-{account}
	secretName := fmt.Sprintf("fio-token-%s", account)

	token, err := retrieveKvSecret(secretName, cred)
	if err != nil {
		log.Printf("[%s] Failed to retrieve token from secret '%s': %v", account, secretName, err)
		return AccountResult{
			Account: account,
			Success: false,
			Error:   fmt.Errorf("failed to retrieve token: %w", err),
		}
	}

	data, err := FetchTransactionData(token, startDate, endDate, format)
	if err != nil {
		log.Printf("[%s] Error fetching data: %v", account, err)
		return AccountResult{
			Account: account,
			Success: false,
			Error:   fmt.Errorf("failed to fetch data: %w", err),
		}
	}

	// Blob name includes account prefix: {account}/transactions_{start}_{end}.json
	blobName := fmt.Sprintf("%s/transactions_%s_%s.json", account, startDate, endDate)

	result, err := writeBlob(*cred, storageContainerName, blobName, data)
	if err != nil {
		log.Printf("[%s] Error writing blob '%s': %v", account, blobName, err)
		return AccountResult{
			Account: account,
			Success: false,
			Error:   fmt.Errorf("failed to write blob: %w", err),
		}
	}

	log.Printf("[%s] Successfully wrote blob: %s", account, blobName)
	return AccountResult{
		Account: account,
		Success: true,
		Message: result,
	}
}

// getDatesFromQuery returns startDate and endDate parsed from the URL query.
// If either is missing, it defaults to yesterday's date.
func getDatesFromQuery(r *http.Request) (string, string) {
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	if startDate == "" || endDate == "" {
		yesterday := time.Now().AddDate(0, 0, -1)
		defaultDate := yesterday.Format("2006-01-02")
		return defaultDate, defaultDate
	}
	return startDate, endDate
}

// azAuth authenticates using DefaultAzureCredential.
func azAuth() (*azidentity.DefaultAzureCredential, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Printf("Failed to create DefaultAzureCredential: %v", err)
		return nil, err
	}
	return cred, nil
}

// retrieveKvSecret retrieves a secret from Azure Key Vault.
func retrieveKvSecret(secretName string, cred *azidentity.DefaultAzureCredential) (string, error) {
	client, err := azsecrets.NewClient(keyVaultURL, cred, nil)
	if err != nil {
		log.Printf("Failed to create KeyVault client: %v\n", err)
		return "", err
	}
	resp, err := client.GetSecret(context.Background(), secretName, "", nil)
	if err != nil {
		log.Printf("KeyVault get secret failed: %v\n", err)
		return "", err
	}
	return *resp.Value, nil
}

// FetchTransactionData makes a GET request to the FIO API.
func FetchTransactionData(token, startDate, endDate, format string) ([]byte, error) {
	fullURL := fmt.Sprintf("https://fioapi.fio.cz/v1/rest/periods/%s/%s/%s/transactions.%s", token, startDate, endDate, format)
	if debug {
		log.Printf("Making API call to: %s\n", fullURL)
	}

	resp, err := httpClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("API returned non-200 status: %d, Response: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read API response: %w", err)
	}
	return body, nil
}

// writeBlob uploads the given data as a blob to the specified container.
func writeBlob(cred azidentity.DefaultAzureCredential, containerName, blobName string, data []byte) (string, error) {
	client, err := azblob.NewClient(storageAccountURL, &cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create blob client: %w", err)
	}
	ctx := context.Background()
	_, err = client.UploadBuffer(ctx, containerName, blobName, data, &azblob.UploadBufferOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to upload blob: %w", err)
	}
	return "Blob uploaded successfully", nil
}

// newHTTPClient returns a reusable HTTP client with a configurable timeout.
func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}
