package docker

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/containers/image/types"
	"github.com/docker/docker/pkg/homedir"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
)

const (
	dockerHostname     = "docker.io"
	dockerRegistry     = "registry-1.docker.io"
	dockerAuthRegistry = "https://index.docker.io/v1/"

	dockerCfg         = ".docker"
	dockerCfgFileName = "config.json"
	dockerCfgObsolete = ".dockercfg"

	baseURL       = "%s://%s/v2/"
	tagsURL       = "%s/tags/list"
	manifestURL   = "%s/manifests/%s"
	blobsURL      = "%s/blobs/%s"
	blobUploadURL = "%s/blobs/uploads/"
)

// dockerClient is configuration for dealing with a single Docker registry.
type dockerClient struct {
	ctx             *types.SystemContext
	registry        string
	username        string
	password        string
	wwwAuthenticate string // Cache of a value set by ping() if scheme is not empty
	scheme          string // Cache of a value returned by a successful ping() if not empty
	client          *http.Client
	signatureBase   signatureStorageBase
}

// this is cloned from docker/go-connections because upstream docker has changed
// it and make deps here fails otherwise.
// We'll drop this once we upgrade to docker 1.13.x deps.
func serverDefault() *tls.Config {
	return &tls.Config{
		// Avoid fallback to SSL protocols < TLS1.0
		MinVersion:               tls.VersionTLS10,
		PreferServerCipherSuites: true,
		CipherSuites:             tlsconfig.DefaultServerAcceptedCiphers,
	}
}

func newTransport() *http.Transport {
	direct := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		Dial:                direct.Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		// TODO(dmcgowan): Call close idle connections when complete and use keep alive
		DisableKeepAlives: true,
	}
	proxyDialer, err := sockets.DialerFromEnvironment(direct)
	if err == nil {
		tr.Dial = proxyDialer.Dial
	}
	return tr
}

// newDockerClient returns a new dockerClient instance for refHostname (a host a specified in the Docker image reference, not canonicalized to dockerRegistry)
// “write” specifies whether the client will be used for "write" access (in particular passed to lookaside.go:toplevelFromSection)
func newDockerClient(ctx *types.SystemContext, ref dockerReference, write bool) (*dockerClient, error) {
	registry := ref.ref.Hostname()
	if registry == dockerHostname {
		registry = dockerRegistry
	}
	username, password, err := getAuth(ctx, ref.ref.Hostname())
	if err != nil {
		return nil, err
	}
	tr := newTransport()
	if ctx != nil && (ctx.DockerCertPath != "" || ctx.DockerInsecureSkipTLSVerify) {
		tlsc := &tls.Config{}

		if ctx.DockerCertPath != "" {
			cert, err := tls.LoadX509KeyPair(filepath.Join(ctx.DockerCertPath, "cert.pem"), filepath.Join(ctx.DockerCertPath, "key.pem"))
			if err != nil {
				return nil, fmt.Errorf("Error loading x509 key pair: %s", err)
			}
			tlsc.Certificates = append(tlsc.Certificates, cert)
		}
		tlsc.InsecureSkipVerify = ctx.DockerInsecureSkipTLSVerify
		tr.TLSClientConfig = tlsc
	}
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = serverDefault()
	}
	client := &http.Client{Transport: tr}

	sigBase, err := configuredSignatureStorageBase(ctx, ref, write)
	if err != nil {
		return nil, err
	}

	return &dockerClient{
		ctx:           ctx,
		registry:      registry,
		username:      username,
		password:      password,
		client:        client,
		signatureBase: sigBase,
	}, nil
}

// makeRequest creates and executes a http.Request with the specified parameters, adding authentication and TLS options for the Docker client.
// url is NOT an absolute URL, but a path relative to the /v2/ top-level API path.  The host name and schema is taken from the client or autodetected.
func (c *dockerClient) makeRequest(method, url string, headers map[string][]string, stream io.Reader) (*http.Response, error) {
	if c.scheme == "" {
		pr, err := c.ping()
		if err != nil {
			return nil, err
		}
		c.wwwAuthenticate = pr.WWWAuthenticate
		c.scheme = pr.scheme
	}

	url = fmt.Sprintf(baseURL, c.scheme, c.registry) + url
	return c.makeRequestToResolvedURL(method, url, headers, stream, -1)
}

// makeRequestToResolvedURL creates and executes a http.Request with the specified parameters, adding authentication and TLS options for the Docker client.
// streamLen, if not -1, specifies the length of the data expected on stream.
// makeRequest should generally be preferred.
func (c *dockerClient) makeRequestToResolvedURL(method, url string, headers map[string][]string, stream io.Reader, streamLen int64) (*http.Response, error) {
	req, err := http.NewRequest(method, url, stream)
	if err != nil {
		return nil, err
	}
	if streamLen != -1 { // Do not blindly overwrite if streamLen == -1, http.NewRequest above can figure out the length of bytes.Reader and similar objects without us having to compute it.
		req.ContentLength = streamLen
	}
	req.Header.Set("Docker-Distribution-API-Version", "registry/2.0")
	for n, h := range headers {
		for _, hh := range h {
			req.Header.Add(n, hh)
		}
	}
	if c.ctx != nil && c.ctx.DockerRegistryUserAgent != "" {
		req.Header.Add("User-Agent", c.ctx.DockerRegistryUserAgent)
	}
	if c.wwwAuthenticate != "" {
		if err := c.setupRequestAuth(req); err != nil {
			return nil, err
		}
	}
	logrus.Debugf("%s %s", method, url)
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *dockerClient) setupRequestAuth(req *http.Request) error {
	tokens := strings.SplitN(strings.TrimSpace(c.wwwAuthenticate), " ", 2)
	if len(tokens) != 2 {
		return fmt.Errorf("expected 2 tokens in WWW-Authenticate: %d, %s", len(tokens), c.wwwAuthenticate)
	}
	switch tokens[0] {
	case "Basic":
		req.SetBasicAuth(c.username, c.password)
		return nil
	case "Bearer":
		// FIXME? This gets a new token for every API request;
		// we may be easily able to reuse a previous token, e.g.
		// for OpenShift the token only identifies the user and does not vary
		// across operations.  Should we just try the request first, and
		// only get a new token on failure?
		// OTOH what to do with the single-use body stream in that case?

		// Try performing the request, expecting it to fail.
		testReq := *req
		// Do not use the body stream, or we couldn't reuse it for the "real" call later.
		testReq.Body = nil
		testReq.ContentLength = 0
		res, err := c.client.Do(&testReq)
		if err != nil {
			return err
		}
		chs := parseAuthHeader(res.Header)
		if res.StatusCode != http.StatusUnauthorized || chs == nil || len(chs) == 0 {
			// no need for bearer? wtf?
			return nil
		}
		// Arbitrarily use the first challenge, there is no reason to expect more than one.
		challenge := chs[0]
		if challenge.Scheme != "bearer" { // Another artifact of trying to handle WWW-Authenticate before it actually happens.
			return fmt.Errorf("Unimplemented: WWW-Authenticate Bearer replaced by %#v", challenge.Scheme)
		}
		realm, ok := challenge.Parameters["realm"]
		if !ok {
			return fmt.Errorf("missing realm in bearer auth challenge")
		}
		service, _ := challenge.Parameters["service"] // Will be "" if not present
		scope, _ := challenge.Parameters["scope"]     // Will be "" if not present
		token, err := c.getBearerToken(realm, service, scope)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		return nil
	}
	return fmt.Errorf("no handler for %s authentication", tokens[0])
	// support docker bearer with authconfig's Auth string? see docker2aci
}

func (c *dockerClient) getBearerToken(realm, service, scope string) (string, error) {
	authReq, err := http.NewRequest("GET", realm, nil)
	if err != nil {
		return "", err
	}
	getParams := authReq.URL.Query()
	if service != "" {
		getParams.Add("service", service)
	}
	if scope != "" {
		getParams.Add("scope", scope)
	}
	authReq.URL.RawQuery = getParams.Encode()
	if c.username != "" && c.password != "" {
		authReq.SetBasicAuth(c.username, c.password)
	}
	tr := newTransport()
	// TODO(runcom): insecure for now to contact the external token service
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{Transport: tr}
	res, err := client.Do(authReq)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusUnauthorized:
		return "", fmt.Errorf("unable to retrieve auth token: 401 unauthorized")
	case http.StatusOK:
		break
	default:
		return "", fmt.Errorf("unexpected http code: %d, URL: %s", res.StatusCode, authReq.URL)
	}
	tokenBlob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	tokenStruct := struct {
		Token string `json:"token"`
	}{}
	if err := json.Unmarshal(tokenBlob, &tokenStruct); err != nil {
		return "", err
	}
	// TODO(runcom): reuse tokens?
	//hostAuthTokens, ok = rb.hostsV2AuthTokens[req.URL.Host]
	//if !ok {
	//hostAuthTokens = make(map[string]string)
	//rb.hostsV2AuthTokens[req.URL.Host] = hostAuthTokens
	//}
	//hostAuthTokens[repo] = tokenStruct.Token
	return tokenStruct.Token, nil
}

func getAuth(ctx *types.SystemContext, registry string) (string, string, error) {
	if ctx != nil && ctx.DockerAuthConfig != nil {
		return ctx.DockerAuthConfig.Username, ctx.DockerAuthConfig.Password, nil
	}
	// TODO(runcom): get this from *cli.Context somehow
	//if username != "" && password != "" {
	//return username, password, nil
	//}
	var dockerAuth dockerConfigFile
	dockerCfgPath := filepath.Join(getDefaultConfigDir(".docker"), dockerCfgFileName)
	if _, err := os.Stat(dockerCfgPath); err == nil {
		j, err := ioutil.ReadFile(dockerCfgPath)
		if err != nil {
			return "", "", err
		}
		if err := json.Unmarshal(j, &dockerAuth); err != nil {
			return "", "", err
		}

	} else if os.IsNotExist(err) {
		// try old config path
		oldDockerCfgPath := filepath.Join(getDefaultConfigDir(dockerCfgObsolete))
		if _, err := os.Stat(oldDockerCfgPath); err != nil {
			if os.IsNotExist(err) {
				return "", "", nil
			}
			return "", "", fmt.Errorf("%s - %v", oldDockerCfgPath, err)
		}

		j, err := ioutil.ReadFile(oldDockerCfgPath)
		if err != nil {
			return "", "", err
		}
		if err := json.Unmarshal(j, &dockerAuth.AuthConfigs); err != nil {
			return "", "", err
		}

	} else if err != nil {
		return "", "", fmt.Errorf("%s - %v", dockerCfgPath, err)
	}

	// I'm feeling lucky
	if c, exists := dockerAuth.AuthConfigs[registry]; exists {
		return decodeDockerAuth(c.Auth)
	}

	// bad luck; let's normalize the entries first
	registry = normalizeRegistry(registry)
	normalizedAuths := map[string]dockerAuthConfig{}
	for k, v := range dockerAuth.AuthConfigs {
		normalizedAuths[normalizeRegistry(k)] = v
	}
	if c, exists := normalizedAuths[registry]; exists {
		return decodeDockerAuth(c.Auth)
	}
	return "", "", nil
}

type apiErr struct {
	Code    string
	Message string
	Detail  interface{}
}

type pingResponse struct {
	WWWAuthenticate string
	APIVersion      string
	scheme          string
	errors          []apiErr
}

func (c *dockerClient) ping() (*pingResponse, error) {
	ping := func(scheme string) (*pingResponse, error) {
		url := fmt.Sprintf(baseURL, scheme, c.registry)
		resp, err := c.makeRequestToResolvedURL("GET", url, nil, nil, -1)
		logrus.Debugf("Ping %s err %#v", url, err)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		logrus.Debugf("Ping %s status %d", scheme+"://"+c.registry+"/v2/", resp.StatusCode)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
			return nil, fmt.Errorf("error pinging repository, response code %d", resp.StatusCode)
		}
		pr := &pingResponse{}
		pr.WWWAuthenticate = resp.Header.Get("WWW-Authenticate")
		pr.APIVersion = resp.Header.Get("Docker-Distribution-Api-Version")
		pr.scheme = scheme
		if resp.StatusCode == http.StatusUnauthorized {
			type APIErrors struct {
				Errors []apiErr
			}
			errs := &APIErrors{}
			if err := json.NewDecoder(resp.Body).Decode(errs); err != nil {
				return nil, err
			}
			pr.errors = errs.Errors
		}
		return pr, nil
	}
	pr, err := ping("https")
	if err != nil && c.ctx != nil && c.ctx.DockerInsecureSkipTLSVerify {
		pr, err = ping("http")
	}
	return pr, err
}

func getDefaultConfigDir(confPath string) string {
	return filepath.Join(homedir.Get(), confPath)
}

type dockerAuthConfig struct {
	Auth string `json:"auth,omitempty"`
}

type dockerConfigFile struct {
	AuthConfigs map[string]dockerAuthConfig `json:"auths"`
}

func decodeDockerAuth(s string) (string, string, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		// if it's invalid just skip, as docker does
		return "", "", nil
	}
	user := parts[0]
	password := strings.Trim(parts[1], "\x00")
	return user, password, nil
}

// convertToHostname converts a registry url which has http|https prepended
// to just an hostname.
// Copied from github.com/docker/docker/registry/auth.go
func convertToHostname(url string) string {
	stripped := url
	if strings.HasPrefix(url, "http://") {
		stripped = strings.TrimPrefix(url, "http://")
	} else if strings.HasPrefix(url, "https://") {
		stripped = strings.TrimPrefix(url, "https://")
	}

	nameParts := strings.SplitN(stripped, "/", 2)

	return nameParts[0]
}

func normalizeRegistry(registry string) string {
	normalized := convertToHostname(registry)
	switch normalized {
	case "registry-1.docker.io", "docker.io":
		return "index.docker.io"
	}
	return normalized
}
