package app

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"ollama-desktop/internal/config"
	"time"
)

func proxyUrl(scheme, host, port, username, password string) *url.URL {
	u := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, port),
	}
	if username != "" {
		if password != "" {
			u.User = url.UserPassword(username, password)
		} else {
			u.User = url.User(username)
		}
	}
	return u
}

func createHttpClient() *http.Client {
	var proxy func(*http.Request) (*url.URL, error)

	var scheme, host, port, username, password string
	if config.Config.Proxy != nil {
		proxy := config.Config.Proxy
		scheme = proxy.Scheme
		host = proxy.Host
		port = proxy.Port
		username = proxy.Username
		password = proxy.Password
	}

	scheme, _ = configStore.getOrDefault(configProxyScheme, scheme)
	host, _ = configStore.getOrDefault(configProxyHost, host)
	port, _ = configStore.getOrDefault(configProxyPort, port)
	username, _ = configStore.getOrDefault(configProxyUsername, username)
	password, _ = configStore.getOrDefault(configProxyPassword, password)

	if scheme != "" && host != "" && port != "" {
		proxy = http.ProxyURL(proxyUrl(scheme, host, port, username, password))
	}
	return &http.Client{
		Timeout: 30 * time.Second, // 设置超时时间为 30 秒
		Transport: &http.Transport{
			Proxy: proxy,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // 不验证证书
			},
		},
	}
}
