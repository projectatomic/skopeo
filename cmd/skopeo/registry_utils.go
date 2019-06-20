package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
)

// Registry stores information about the targeted docker registry to
// to optimize the numerous calls to registry API
type Registry struct {
	baseURL      string
	catalogURL   string
	authType     string
	regUsername  string
	regPassword  string
	repos        []string
	catalogToken string
}

const (
	tokenAuth string = "token"
	basicAuth string = "basic"
	noneAuth  string = "none"
)

// NewRegistry creates a new Registry struct with parsed/sanitized URLs
func NewRegistry(rawURL string) (Registry, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return Registry{}, fmt.Errorf("Failed to parse Registry URL: %v", err)
	}
	baseURL := parsedURL.String()
	parsedURL.Path = path.Join(parsedURL.Path, "v2/_catalog")
	catalogURL := parsedURL.String()

	return Registry{baseURL: baseURL, catalogURL: catalogURL}, nil
}

// Determines auth type to optimize later registry calls
func (r *Registry) getAuthType() error {
	resp, err := http.Get(r.catalogURL)
	if err != nil {
		return fmt.Errorf("Failed to connect to Catalog URL: %v", err)
	}

	headerAuth := resp.Header.Get("www-authenticate")
	headerAuthLower := strings.ToLower(headerAuth)

	if strings.Count(headerAuthLower, "bearer") > 0 {
		fmt.Println("Using Token Auth")
		r.authType = tokenAuth
	} else if strings.Count(headerAuthLower, "basic") > 0 {
		fmt.Println("Using Basic Auth")
		r.authType = basicAuth
	} else {
		fmt.Println("Using No Auth")
		r.authType = noneAuth
	}
	return nil
}

// Uses r.authType to abstract Authentication
func (r *Registry) applyAuth(req *http.Request) error {
	if r.authType == tokenAuth {
		var token string
		// Each catalog request returns a limit of 100 repos at a time.
		// This conditional saves a few requests for large registry catalogs.
		// By resetting r.catalogToken at end of getV2Catalog(), this
		// can also be used to handle auth for getting tags.
		// This could could be improved.
		if len(r.catalogToken) > 0 {
			token = r.catalogToken
		} else {
			// err must be declared to prevent token from being re-declared with ":=" below
			var err error
			token, err = getToken(req.URL.String())
			if err != nil {
				return err
			}
			r.catalogToken = token
		}
		req.Header.Set("Authorization", "Bearer "+token)
	} else if r.authType == basicAuth {
		req.SetBasicAuth(r.regUsername, r.regPassword)
	} else {
		// Do Nothing
	}
	return nil
}

// Prints list of registry repos with all available tags
func (r *Registry) getAllReposWithTags() error {
	err := r.getAuthType()
	if err != nil {
		return err
	}
	r.getV2Catalog()

	var wg sync.WaitGroup
	var lock sync.Mutex
	wg.Add(len(r.repos))
	reposWithTags := make(map[string][]string)

	// Alphabetize repos
	sort.Strings(r.repos)

	for _, repo := range r.repos {
		go func(repo string) {
			defer wg.Done()
			tags, _ := r.getRepoTags(repo)
			lock.Lock()
			reposWithTags[repo] = tags
			lock.Unlock()
		}(repo)
	}
	wg.Wait()

	// Display repos with tags - format to be optimized later
	// Consider using https://github.com/olekukonko/tablewriter
	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 10, 0, 1, ' ', tabwriter.TabIndent|tabwriter.Debug)
	fmt.Fprintln(w, "\t REPO \t TAGS \t")
	fmt.Fprintln(w, "\t - - - - - - - - - - \t - - - - - - - - - - \t")
	for _, repo := range r.repos {
		tags := strings.Join(reposWithTags[repo], "  ")
		fmt.Fprintf(w, "\t %s\t %v\t\n", repo, tags)
	}
	w.Flush()
	return nil
}

// Gets repo catalog from a registry
func (r *Registry) getV2Catalog() error {
	req, err := http.NewRequest("GET", r.catalogURL, nil)
	if err != nil {
		return fmt.Errorf("Failed to build requset when retrieving repo catalog: %v", err)
	}

	r.applyAuth(req)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to connect to Catalog URL: %v", err)
	}

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("Failed to parse Catalog URL response body: %v", err)
	}

	var repositories []string
	var replacer = strings.NewReplacer("<", "", ">", "")

	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		var f map[string][]string
		err = json.Unmarshal(bodyBytes, &f)
		if err != nil {
			return fmt.Errorf("Failed to unmarshal Catalog URL response body: %v", err)
		}
		repositories = append(repositories, f["repositories"]...)
		// Look for more paginated repo link in header and follow
		headerLink := res.Header.Get("Link")
		for len(headerLink) != 0 {
			guardedLink := strings.Split(headerLink, ";")[0]
			cleanedLink := replacer.Replace(guardedLink)
			parsedBaseURL, err := url.Parse(r.baseURL)
			if err != nil {
				return fmt.Errorf("Failed to parse link URL from Catalog header: %v", err)
			}
			parsedBaseURL.Path = path.Join(parsedBaseURL.Path, cleanedLink)
			unescapedURL, err := url.QueryUnescape(parsedBaseURL.String())
			if err != nil {
				return fmt.Errorf("Failed to unescape link URL from Catalog header: %v", err)
			}
			req2, err := http.NewRequest("GET", unescapedURL, nil)
			if err != nil {
				return fmt.Errorf("Failed to build requset when retrieving repo catalog: %v", err)
			}
			r.applyAuth(req2)
			res2, err := http.DefaultClient.Do(req2)
			if err != nil {
				return fmt.Errorf("Failed to connect to Catalog URL: %v", err)
			}
			bodyBytes2, err := ioutil.ReadAll(res2.Body)
			if err != nil {
				return fmt.Errorf("Failed to parse Catalog URL response body: %v", err)
			}
			err = json.Unmarshal(bodyBytes2, &f)
			if err != nil {
				return fmt.Errorf("Failed to unmarshal Catalog URL response body: %v", err)
			}
			repositories = append(repositories, f["repositories"]...)
			headerLink = res2.Header.Get("Link")
		}
	} else {
		fmt.Println("Failed to get repository list")
	}

	r.repos = repositories
	r.catalogToken = ""
	return nil
}

// Gets tags for a repo
func (r *Registry) getRepoTags(repo string) ([]string, error) {
	parsedURL, err := url.Parse(r.baseURL)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse URL when getting tags for %s: %v", repo, err)
	}
	parsedURL.Path = path.Join(parsedURL.Path, "v2/", repo, "/tags/list")
	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to build requset when retrieving tags for %s: %v", repo, err)
	}
	r.applyAuth(req)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to tag URL for %s: %v", repo, err)
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse tag URL response body for %s: %v", repo, err)
	}
	var f map[string][]string
	// Unmarshal responds with errors here even when output appears OK
	_ = json.Unmarshal(bodyBytes, &f)

	return f["tags"], nil
}

// Gets Bearer Token for registries using Token Authentication
func getToken(baseURL string) (string, error) {
	// Request to gather auth parameters
	resp, err := http.Get(baseURL)
	if err != nil {
		return "", fmt.Errorf("Failed to connect to Base URL: %v", err)
	}

	headerAuth := resp.Header.Get("www-authenticate")

	headerAuthSlice := strings.Split(headerAuth, ",")
	var authRealm, authService, authScope string
	for _, elem := range headerAuthSlice {
		if strings.Count(strings.ToLower(elem), "bearer") == 1 {
			elem = strings.Split(elem, " ")[1]
		}
		elem := strings.Replace(elem, "\"", "", -1)
		elemSplit := strings.Split(elem, "=")
		if len(elemSplit) != 2 {
			return "", fmt.Errorf("Incorrectly formatted Header Auth: %s", headerAuth)
		}
		authKey := elemSplit[0]
		authValue := elemSplit[1]
		switch authKey {
		case "realm":
			authRealm = authValue
		case "service":
			authService = authValue
		case "scope":
			authScope = authValue
		}
	}

	parsedRealm, err := url.Parse(authRealm)
	if err != nil {
		return "", fmt.Errorf("Failed to parse Auth Realm URL: %v", err)
	}

	// Build query for token
	q := parsedRealm.Query()
	q.Set("service", authService)
	q.Set("scope", authScope)
	parsedRealm.RawQuery = q.Encode()

	// Make request for token
	resp2, err := http.Get(parsedRealm.String())
	if err != nil {
		return "", fmt.Errorf("Failed to connect to Auth Realm URL: %v", err)
	}

	// Extract token
	body, err := ioutil.ReadAll(resp2.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to parse token URL response body: %v", err)
	}
	var f map[string]string
	err = json.Unmarshal(body, &f)
	if err != nil {
		return "", fmt.Errorf("Failed to unmarshal token URL response body: %v", err)
	}
	token := f["token"]

	return token, nil
}
