package core

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/valyala/fasthttp"
	"log"
	"megafin_farmer/config"
	"megafin_farmer/customTypes"
	"megafin_farmer/metrics"
	"strconv"
	"strings"
	"time"
)

func doRequest(client *fasthttp.Client,
	url string,
	method string,
	payload interface{},
	headers map[string]string) ([]byte, int, error) {

	metrics.IsServerDown()

	metrics.TotalRequests.WithLabelValues(method, "attempt").Inc()
	start := time.Now()

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetMethod(strings.ToUpper(method))
	req.SetRequestURI(url)
	req.Header.SetContentType("application/json")

	var requestSize int64 = 0

	if payload != nil {
		jsonData, err := json.Marshal(payload)

		if err != nil {
			metrics.ErrorCounter.WithLabelValues("json_marshal").Inc()
			return nil, 0, fmt.Errorf("failed to marshal JSON: %w", err)
		}
		req.SetBody(jsonData)
		requestSize = int64(len(jsonData))
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	req.Header.VisitAll(func(key, value []byte) {
		requestSize += int64(len(key) + len(value))
	})
	metrics.TotalTrafficBytes.WithLabelValues("out").Add(float64(requestSize))

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	if err := client.Do(req, resp); err != nil {
		metrics.TotalErrors.WithLabelValues("request_failed").Inc()
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}

	statusCode := resp.StatusCode()
	metrics.TotalRequests.WithLabelValues(method, strconv.Itoa(statusCode)).Inc()
	if statusCode == 520 {
		metrics.SetServerDown()
		return nil, statusCode, fmt.Errorf("server is down (520 error)")
	}
	metrics.SetServerUp()

	metrics.RequestDuration.WithLabelValues(method).Observe(time.Since(start).Seconds())
	metrics.ResponseStatus.WithLabelValues(method, strconv.Itoa(statusCode)).Inc()
	if statusCode >= 400 {
		metrics.TotalErrors.WithLabelValues("http_" + strconv.Itoa(statusCode)).Inc()
	}

	respBody := make([]byte, len(resp.Body()))
	copy(respBody, resp.Body())

	responseSize := int64(len(respBody))
	resp.Header.VisitAll(func(key, value []byte) {
		responseSize += int64(len(key) + len(value))
	})
	metrics.TotalTrafficBytes.WithLabelValues("in").Add(float64(responseSize))

	return respBody, resp.StatusCode(), nil
}

func profileRequest(client *fasthttp.Client,
	privateKeyHex string,
	headers map[string]string) (map[string]string, float64, float64) {
	for {
		var responseData customTypes.ProfileResponseStruct

		respBody, statusCode, err := doRequest(client, "https://api.megafin.xyz/users/profile", "GET", nil, headers)

		if err != nil {
			log.Printf("%s | Error When Profile: %s | Status Code: %d", privateKeyHex, err, statusCode)
			headers = config.GlobalHeadersManager.ReplaceHeadersForAccount(privateKeyHex, headers)
			continue
		}

		if strings.Contains(string(respBody), "title>Access denied | api.megafin.xyz used Cloudflare to restrict access</title>") || strings.Contains(string(respBody), "<title>Just a moment...</title>") || strings.Contains(string(respBody), "<title>Attention Required! | Cloudflare</title>\n") {
			log.Printf("%s | CloudFlare", privateKeyHex)
			headers = config.GlobalHeadersManager.ReplaceHeadersForAccount(privateKeyHex, headers)
			continue
		}

		if err = json.Unmarshal(respBody, &responseData); err != nil {
			log.Printf("%s | Failed To Parse JSON Response When Profile: %s | Status Code: %d", privateKeyHex, string(respBody), statusCode)
			headers = config.GlobalHeadersManager.ReplaceHeadersForAccount(privateKeyHex, headers)
			continue
		}

		return headers, responseData.Result.Balance.MGF, responseData.Result.Balance.USDC
	}
}

func loginAccount(client *fasthttp.Client,
	privateKeyHex string,
	headers map[string]string) (map[string]string, string) {

	headers["accept"] = "application/json"

	privateKey, err := crypto.HexToECDSA(privateKeyHex)

	if err != nil {
		log.Panicf("%s | Failed to parse private key: %v", privateKeyHex, err)
	}

	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	signText := fmt.Sprintf("megafin.xyz requests you to sign in with your wallet address: %s", address.Hex())
	data := accounts.TextHash([]byte(signText))
	signature, err := crypto.Sign(data, privateKey)

	if err != nil {
		log.Panicf("%s | Failed to sign message: %v", privateKeyHex, err)
	}

	signature[64] += 27
	signHash := fmt.Sprintf("0x%x", signature)

	payload := map[string]interface{}{
		"invite_code": config.GlobalConfig.RefCode,
		"key":         address.String(),
		"wallet_hash": signHash,
	}

	for {
		var responseData customTypes.LoginResponseStruct
		respBody, statusCode, err := doRequest(client, "https://api.megafin.xyz/auth", "POST", payload, headers)

		if err != nil {
			log.Printf("%s | Error When Auth: %s | Status Code: %d", privateKeyHex, err, statusCode)
			headers = config.GlobalHeadersManager.ReplaceHeadersForAccount(privateKeyHex, headers)
			continue
		}

		if strings.Contains(string(respBody), "title>Access denied | api.megafin.xyz used Cloudflare to restrict access</title>") || strings.Contains(string(respBody), "<title>Just a moment...</title>") {
			log.Printf("%s | CloudFlare", privateKeyHex)
			headers = config.GlobalHeadersManager.ReplaceHeadersForAccount(privateKeyHex, headers)
			continue
		}

		if err = json.Unmarshal(respBody, &responseData); err != nil {
			log.Printf("%s | Failed To Parse JSON Response When Logging: %s | Status Code: %d", privateKeyHex, string(respBody), statusCode)
			headers = config.GlobalHeadersManager.ReplaceHeadersForAccount(privateKeyHex, headers)
			continue
		}

		return headers, responseData.Result.Token
	}
}

func sendConnectRequest(client *fasthttp.Client,
	privateKeyHex string,
	headers map[string]string) (map[string]string, float64, float64) {
	for {
		var responseData customTypes.PingResponseStruct

		respBody, statusCode, err := doRequest(client, "https://api.megafin.xyz/users/connect", "GET", nil, headers)

		if err != nil {
			log.Printf("%s | Error When Pinging: %s | Status Code: %d", privateKeyHex, err, statusCode)
			headers = config.GlobalHeadersManager.ReplaceHeadersForAccount(privateKeyHex, headers)
			continue
		}

		if strings.Contains(string(respBody), "title>Access denied | api.megafin.xyz used Cloudflare to restrict access</title>") || strings.Contains(string(respBody), "<title>Just a moment...</title>") || strings.Contains(string(respBody), "<title>Attention Required! | Cloudflare</title>\n") {
			log.Printf("%s | CloudFlare", privateKeyHex)
			headers = config.GlobalHeadersManager.ReplaceHeadersForAccount(privateKeyHex, headers)
			continue
		}

		if err = json.Unmarshal(respBody, &responseData); err != nil {
			log.Printf("%s | Failed To Parse JSON Response When Pinging: %s | Status Code: %d", privateKeyHex, string(respBody), statusCode)
			headers = config.GlobalHeadersManager.ReplaceHeadersForAccount(privateKeyHex, headers)
			continue
		}

		return headers, responseData.Result.Balance.MGF, responseData.Result.Balance.USDC
	}
}

func StartFarmAccount(privateKey string,
	proxy string) {
	headers := config.GlobalHeadersManager.GetHeadersForAccount(privateKey)
	metrics.IncrementActiveAccounts()
	defer metrics.DecrementActiveAccounts()
	client := GetClient(proxy)
	headers, authToken := loginAccount(client, privateKey, headers)
	headers["Authorization"] = "Bearer " + authToken
	profileRequest(client, privateKey, headers)

	for {
		var mgfBalance, usdcBalance float64
		headers, mgfBalance, usdcBalance = sendConnectRequest(client, privateKey, headers)

		metrics.UpdateAccountBalance(privateKey, mgfBalance, usdcBalance)

		log.Printf("%s | MGF Balance: %f | USDC Balance: %f | Sleeping 90 secs.",
			privateKey, mgfBalance, usdcBalance)

		isServerDown := metrics.IsServerDown()

		if isServerDown {
			log.Printf("%s | Server is down, waiting for 5 minutes", privateKey)
			time.Sleep(5 * time.Minute)
			continue
		}

		time.Sleep(time.Second * time.Duration(90))
	}
}

func ParseAccountBalance(privateKey string,
	proxy string) (float64, float64) {
	headers := config.GlobalHeadersManager.GetHeadersForAccount(privateKey)

	client := GetClient(proxy)
	headers, authToken := loginAccount(client, privateKey, headers)
	headers["Authorization"] = "Bearer " + authToken
	headers, mgfBalance, usdcBalance := profileRequest(client, privateKey, headers)

	metrics.UpdateAccountBalance(privateKey, mgfBalance, usdcBalance)

	log.Printf("%s | MGF Balance: %f | USDC Balance: %f", privateKey, mgfBalance, usdcBalance)

	return mgfBalance, usdcBalance
}
