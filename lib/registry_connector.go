package lib

import (
	"net/http"
	"net/url"
	"strings"

	"git.mayflower.de/vaillant-team/docker-ls/lib/auth"
)

type registryConnector struct {
	cfg           Config
	httpClient    *http.Client
	authenticator auth.Authenticator
	semaphore     chan int
}

func (r *registryConnector) AquireLock() {
	r.semaphore <- 1
}

func (r *registryConnector) ReleaseLock() {
	_ = <-r.semaphore
}

func (r *registryConnector) Get(url *url.URL) (response *http.Response, err error) {
	r.AquireLock()
	defer r.ReleaseLock()

	request, err := http.NewRequest("GET", url.String(), strings.NewReader(""))

	if err != nil {
		return
	}

	resp, err := r.httpClient.Do(request)

	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		response = resp
		return
	}

	if resp.Close {
		resp.Body.Close()
	}

	challenge, err := auth.ParseChallenge(resp.Header.Get("www-authenticate"))

	if err != nil {
		return
	}

	token, err := r.authenticator.PerformRequest(challenge)

	if err != nil {
		return
	}

	request.Header.Set("Authorization", "Bearer "+token)

	response, err = r.httpClient.Do(request)

	return
}

func NewRegistryConnector(cfg Config) *registryConnector {
	connector := registryConnector{
		cfg:        cfg,
		httpClient: http.DefaultClient,
		semaphore:  make(chan int, cfg.maxConcurrentRequests),
	}

	connector.authenticator = auth.NewAuthenticator(connector.httpClient, &cfg.credentials)

	return &connector
}
