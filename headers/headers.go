package headers

import (
	"encoding/json"
	"fmt"
	"github.com/valyala/fasthttp"
	"log"
	"sync"
	"time"
)

type Manager struct {
	headers     []map[string]string
	usedHeaders map[string]map[string]string
	mu          sync.RWMutex
	apiKey      string
	httpClient  *fasthttp.Client
}

func NewHeadersManager(apiKey string, client *fasthttp.Client) *Manager {
	log.Println("Initializing Headers Manager...")
	return &Manager{
		headers:     make([]map[string]string, 0, 1000),
		usedHeaders: make(map[string]map[string]string),
		apiKey:      apiKey,
		httpClient:  client,
	}
}

func (m *Manager) PrepareHeadersForAccounts(accountCount int) error {
	start := time.Now()
	log.Printf("Preparing headers for %d accounts...\n", accountCount)

	fetchCount := (accountCount / 100) + 1
	//log.Printf("Will fetch %d batches of headers\n", fetchCount)

	var wg sync.WaitGroup
	errorChan := make(chan error, fetchCount)
	successChan := make(chan bool, fetchCount)

	for i := 0; i < fetchCount; i++ {
		wg.Add(1)
		go func(batchNum int) {
			defer wg.Done()

			//log.Printf("Starting header batch %d\n", batchNum)
			//batchStart := time.Now()

			err := m.fetchAdditionalHeaders()
			if err != nil {
				log.Printf("Header batch %d failed: %v\n", batchNum, err)
				errorChan <- err
				return
			}

			//duration := time.Since(batchStart)
			//log.Printf(" Batch %d completed in %v\n", batchNum, duration)
			successChan <- true
		}(i + 1)
	}

	go func() {
		wg.Wait()
		close(errorChan)
		close(successChan)
	}()

	successCount := 0
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	for range successChan {
		successCount++
	}

	if len(errors) > 0 {
		log.Printf(" Encountered %d errors during headers preparation\n", len(errors))
		return fmt.Errorf("errors during headers preparation: %v", errors)
	}

	totalHeadersCount := m.HeadersCount()
	duration := time.Since(start)
	log.Printf("Headers Preparation Complete: Fetched %d headers in %v\n",
		totalHeadersCount, duration)

	return nil
}

func (m *Manager) fetchAdditionalHeaders() error {
	url := fmt.Sprintf(
		"https://headers.scrapeops.io/v1/browser-headers?api_key=%s&num_results=100",
		m.apiKey,
	)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod("GET")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	err := m.httpClient.Do(req, resp)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("bad status: %d", resp.StatusCode())
	}

	var response struct {
		Result []map[string]string `json:"result"`
	}

	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		return fmt.Errorf("json parse error: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, header := range response.Result {
		customHeader := map[string]string{
			"accept":     "*/*",
			"origin":     "https://app.megafin.xyz",
			"referer":    "https://app.megafin.xyz",
			"user-agent": header["user-agent"],
		}

		for key, value := range header {
			if key != "user-agent" && key != "accept" {
				customHeader[key] = value
			}
		}

		m.headers = append(m.headers, customHeader)
	}

	return nil
}

func (m *Manager) HeadersCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.headers)
}

func (m *Manager) GetHeadersForAccount(privateKey string) map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()

	//log.Printf("Getting headers for account: %s\n", privateKey)

	if headers, exists := m.usedHeaders[privateKey]; exists {
		log.Printf("Returning cached headers for %s\n", privateKey)
		return headers
	}

	if len(m.headers) == 0 {
		log.Println("No headers available, fetching emergency headers...")

		if err := m.fetchAdditionalHeaders(); err != nil {
			log.Printf(" Critical: Failed to fetch headers: %v\n", err)

			defaultHeaders := map[string]string{
				"accept":     "*/*",
				"origin":     "https://app.megafin.xyz",
				"referer":    "https://app.megafin.xyz",
				"user-agent": "Mozilla/5.0",
			}

			return defaultHeaders
		}
	}

	headers := m.headers[0]
	m.headers = m.headers[1:]
	m.usedHeaders[privateKey] = headers

	//log.Printf("Assigned new headers to %s\n", privateKey)
	return headers
}

func (m *Manager) ReplaceHeadersForAccount(privateKey string, currentHeaders map[string]string) map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("Replacing headers for account: %s\n", privateKey)

	if len(m.headers) == 0 {
		log.Println("No headers for replacement, fetching emergency headers...")

		if err := m.fetchAdditionalHeaders(); err != nil {
			log.Printf("Critical: Failed to fetch headers: %v\n", err)
			return currentHeaders
		}
	}

	newHeaders := m.headers[0]
	m.headers = m.headers[1:]

	if authToken, exists := currentHeaders["Authorization"]; exists {
		newHeaders["Authorization"] = authToken
		log.Println("Preserved Authorization token")
	}

	m.usedHeaders[privateKey] = newHeaders

	log.Printf("Successfully replaced headers for %s\n", privateKey)
	return newHeaders
}
