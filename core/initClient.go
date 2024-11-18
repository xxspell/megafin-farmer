package core

import (
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
	"log"
	"net/url"
	"time"
)

func GetClient(currentProxy string) *fasthttp.Client {
	var dial fasthttp.DialFunc

	if currentProxy != "" {
		proxy, err := url.Parse(currentProxy)
		if err != nil {
			log.Panicf("Error Unparsing Proxy: %v\n", err)
		}

		switch proxy.Scheme {
		case "http", "https":
			dial = fasthttpproxy.FasthttpHTTPDialer(proxy.String())

		case "socks4":
			dial = fasthttpproxy.FasthttpSocksDialer(proxy.String())
		case "socks5":
			dial = fasthttpproxy.FasthttpSocksDialer(proxy.String())
		default:
			log.Panicf("Unsupported proxy scheme: %s\n", proxy.Scheme)
		}
	}

	client := &fasthttp.Client{
		Dial:                          dial,
		MaxConnsPerHost:               0,
		MaxIdleConnDuration:           90 * time.Second,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		ReadTimeout:                   30 * time.Second,
		WriteTimeout:                  30 * time.Second,
		MaxConnWaitTimeout:            30 * time.Second,
		StreamResponseBody:            true,
	}

	return client
}
