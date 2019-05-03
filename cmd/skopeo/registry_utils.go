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
func NewRegistry(rawURL string) Registry {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		fmt.Errorf("Failed to parse URL: %v", err)
	}
	baseURL := parsedURL.String()
	parsedURL.Path = path.Join(parsedURL.Path, "v2/_catalog")
	catalogURL := parsedURL.String()

	return Registry{baseURL: baseURL, catalogURL: catalogURL}
}

// Determines auth type to optimize later registry calls
func (r *Registry) getAuthType() {
	resp, err := http.Get(r.catalogURL)
	if err != nil {
		fmt.Errorf("Failed to connect to Catalog URL: %v", err)
	}

	headerAuth := resp.Header.Get("www-authenticate")
	headerAuthLower := strings.ToLower(headerAuth)

	if strings.Count(headerAuthLower, "bearer") > 0 {
		fmt.Println("Using Bearer Auth")
		r.authType = tokenAuth
	} else if strings.Count(headerAuthLower, "basic") > 0 {
		fmt.Println("Using Basic Auth")
		r.authType = basicAuth
	} else {
		fmt.Println("Using No Auth")
		r.authType = noneAuth
	}
}

// Uses r.authType to abstract Authentication
func (r *Registry) applyAuth(req *http.Request) {
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
			token = getToken(req.URL.String())
			r.catalogToken = token
		}
		req.Header.Set("Authorization", "Bearer "+token)
	} else if r.authType == basicAuth {
		req.SetBasicAuth(r.regUsername, r.regPassword)
	} else {
		// Do Nothing
	}
}

// Prints list of registry repos with all available tags
func (r *Registry) getAllReposWithTags() {
	r.getAuthType()
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
			tags := r.getRepoTags(repo)
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
		// Shorten tag output for testing
		// if len(tags) > 45 {
		// 	fmt.Fprintf(w, "\t %s\t %v\t\n", repo, tags[:45])
		// } else {
		// 	fmt.Fprintf(w, "\t %s\t %v\t\n", repo, tags)
		// }
		fmt.Fprintf(w, "\t %s\t %v\t\n", repo, tags)
	}
	w.Flush()
}

// Gets repo catalog from a registry
func (r *Registry) getV2Catalog() {
	req, err := http.NewRequest("GET", r.catalogURL, nil)
	if err != nil {
		fmt.Errorf("Failed to connect to Catalog URL: %v", err)
	}

	r.applyAuth(req)

	res, _ := http.DefaultClient.Do(req)
	bodyBytes, _ := ioutil.ReadAll(res.Body)

	var repositories []string
	var replacer = strings.NewReplacer("<", "", ">", "")

	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		var f map[string][]string
		_ = json.Unmarshal(bodyBytes, &f)
		repositories = append(repositories, f["repositories"]...)
		// Look for more paginated repo link in header and follow
		headerLink := res.Header.Get("Link")
		for len(headerLink) != 0 {
			guardedLink := strings.Split(headerLink, ";")[0]
			cleanedLink := replacer.Replace(guardedLink)
			parsedBaseURL, _ := url.Parse(r.baseURL)
			parsedBaseURL.Path = path.Join(parsedBaseURL.Path, cleanedLink)
			unescapedURL, _ := url.QueryUnescape(parsedBaseURL.String())
			req2, _ := http.NewRequest("GET", unescapedURL, nil)
			r.applyAuth(req2)
			res2, _ := http.DefaultClient.Do(req2)
			bodyBytes2, _ := ioutil.ReadAll(res2.Body)
			_ = json.Unmarshal(bodyBytes2, &f)
			repositories = append(repositories, f["repositories"]...)
			headerLink = res2.Header.Get("Link")
		}
	} else {
		fmt.Println("Failed to get repository list")
	}

	// fmt.Println(len(repositories))  // Remove later
	r.repos = repositories
	r.catalogToken = ""
}

// Gets tags for a repo
func (r *Registry) getRepoTags(repo string) []string {
	parsedURL, _ := url.Parse(r.baseURL)
	parsedURL.Path = path.Join(parsedURL.Path, "v2/", repo, "/tags/list")
	req, _ := http.NewRequest("GET", parsedURL.String(), nil)
	r.applyAuth(req)
	res, _ := http.DefaultClient.Do(req)
	bodyBytes, _ := ioutil.ReadAll(res.Body)
	var f map[string][]string
	_ = json.Unmarshal(bodyBytes, &f)

	return f["tags"]
}

// Gets Bearer Token for registries using Token Authentication
func getToken(baseURL string) string {
	// Request to gather auth parameters
	resp, err := http.Get(baseURL)
	if err != nil {
		fmt.Errorf("Failed to connect to Base URL: %v", err)
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
			fmt.Printf("Incorrectly formatted Header Auth: %s\n", headerAuth)
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
		fmt.Errorf("Failed to parse Auth Realm URL: %v", err)
	}

	// Build query for token
	q := parsedRealm.Query()
	q.Set("service", authService)
	q.Set("scope", authScope)
	parsedRealm.RawQuery = q.Encode()

	// Make request for token
	resp2, err := http.Get(parsedRealm.String())
	if err != nil {
		fmt.Errorf("Failed to connect to Auth Realm URL: %v", err)
	}

	// Extract token
	body, _ := ioutil.ReadAll(resp2.Body)
	var f map[string]string
	_ = json.Unmarshal(body, &f)
	token := f["token"]

	return token
}
