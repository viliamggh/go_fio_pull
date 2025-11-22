package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

const format = "json"

var (
	debug                = os.Getenv("DEBUG") != ""
	keyVaultURL          = getEnvOrDefault("KEY_VAULT_URL", "https://kv-fintrack-dev.vault.azure.net")
	storageAccountURL    = getEnvOrDefault("STORAGE_ACCOUNT_URL", "https://safintrackdev.blob.core.windows.net/")
	storageContainerName = getEnvOrDefault("STORAGE_CONTAINER_NAME", "raw")
)

// getEnvOrDefault retrieves an environment variable or returns a default value.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	log.Printf("Warning: %s not set, using default: %s", key, defaultValue)
	return defaultValue
}

func main() {
	// Configure the logger for local development.
	// In production you may want to redirect logs to a file or disable debug logs.
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	http.HandleFunc("/", handler)
	log.Println("Server is starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// handler fetches transaction data from FIO API and writes it to Azure Blob Storage.
func handler(w http.ResponseWriter, r *http.Request) {
	cred, err := azAuth()
	if err != nil {
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	startDate, endDate := getDatesFromQuery(r)
	if debug {
		log.Printf("Using startDate=%s and endDate=%s\n", startDate, endDate)
	}

	token, err := retrieveKvSecret("fio-read-token", cred)
	if err != nil {
		http.Error(w, "Failed to retrieve secret", http.StatusInternalServerError)
		return
	}

	data, err := FetchTransactionData(token, startDate, endDate, format)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching data: %v", err), http.StatusInternalServerError)
		return
	}

	// Construct a blob name and upload the API response.
	blobName := fmt.Sprintf("transactions_%s_%s.json", startDate, endDate)
	result, err := writeBlob(*cred, storageContainerName, blobName, data)
	if err != nil {
		log.Printf("Error writing blob: %v\n", err)
		fmt.Fprintf(w, "\nError writing blob: %v", err)
		return
	}
	fmt.Fprintf(w, "\n%s", result)
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

	resp, err := http.Get(fullURL)
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
