package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sync"
	"sync/atomic"
	"time"
)

type AccountBalance struct {
	MGF  float64
	USDC float64
}

var (
	accountBalances = make(map[string]AccountBalance)
	balanceMutex    sync.RWMutex
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "megafin_request_duration_seconds",
			Help:    "Duration of requests in seconds",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10), // от 10ms до ~10s
		},
		[]string{"method"},
	)

	ResponseStatus = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "megafin_response_status",
			Help: "Counter of response status codes",
		},
		[]string{"method", "status"},
	)

	ErrorCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "megafin_errors_total",
			Help: "Counter of different types of errors",
		},
		[]string{"type"},
	)

	ActiveAccounts = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "megafin_active_accounts_total",
		Help: "Total number of active accounts",
	})

	TotalTrafficBytes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "megafin_total_traffic_bytes",
			Help: "Total traffic in bytes across all accounts",
		},
		[]string{"direction"},
	)

	TotalRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "megafin_total_requests",
			Help: "Total number of requests across all accounts",
		},
		[]string{"method", "status"},
	)

	TotalErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "megafin_total_errors",
			Help: "Total number of errors across all accounts",
		},
		[]string{"type"},
	)

	TotalMgfBalance = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "megafin_total_mgf_balance",
			Help: "Total MGF balance across all accounts",
		},
	)

	TotalUsdcBalance = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "megafin_total_usdc_balance",
			Help: "Total USDC balance across all accounts",
		},
	)

	ServerStatus = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "megafin_server_status",
			Help: "Server status (1 = up, 0 = down)",
		},
	)
)

var (
	serverDownTime      time.Time
	serverDownMutex     sync.RWMutex
	activeAccountsCount int32
)

func UpdateAccountBalance(privateKey string, mgf, usdc float64) {
	balanceMutex.Lock()
	defer balanceMutex.Unlock()

	accountBalances[privateKey] = AccountBalance{
		MGF:  mgf,
		USDC: usdc,
	}

	var totalMGF, totalUSDC float64
	for _, balance := range accountBalances {
		totalMGF += balance.MGF
		totalUSDC += balance.USDC
	}

	TotalMgfBalance.Set(totalMGF)
	TotalUsdcBalance.Set(totalUSDC)
}

func IncrementActiveAccounts() {
	count := atomic.AddInt32(&activeAccountsCount, 1)
	ActiveAccounts.Set(float64(count))
}

func DecrementActiveAccounts() {
	count := atomic.AddInt32(&activeAccountsCount, -1)
	ActiveAccounts.Set(float64(count))
}

func SetServerDown() {
	serverDownMutex.Lock()
	defer serverDownMutex.Unlock()
	serverDownTime = time.Now()
	ServerStatus.Set(0)
}

func SetServerUp() {
	serverDownMutex.Lock()
	defer serverDownMutex.Unlock()
	serverDownTime = time.Time{}
	ServerStatus.Set(1)
}

func IsServerDown() bool {
	serverDownMutex.RLock()
	defer serverDownMutex.RUnlock()
	return !serverDownTime.IsZero() && time.Since(serverDownTime) < 5*time.Minute
}
