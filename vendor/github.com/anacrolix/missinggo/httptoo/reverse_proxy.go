package httptoo

import (
	"bufio"
	"encoding/gob"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
)

func deepCopy(dst, src interface{}) error {
	r, w := io.Pipe()
	e := gob.NewEncoder(w)
	d := gob.NewDecoder(r)
	var decErr, encErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		decErr = d.Decode(dst)
		r.Close()
	}()
	encErr = e.Encode(src)
	// Always returns nil.
	w.CloseWithError(encErr)
	wg.Wait()
	if encErr != nil {
		return encErr
	}
	return decErr
}

// Takes a request, and alters its destination fields, for proxying.
func RedirectedRequest(r *http.Request, newUrl string) (ret *http.Request, err error) {
	u, err := url.Parse(newUrl)
	if err != nil {
		return
	}
	ret = new(http.Request)
	*ret = *r
	ret.Header = nil
	err = deepCopy(&ret.Header, r.Header)
	if err != nil {
		return
	}
	ret.URL = u
	ret.RequestURI = ""
	return
}

func CopyHeaders(w http.ResponseWriter, r *http.Response) {
	for h, vs := range r.Header {
		for _, v := range vs {
			w.Header().Add(h, v)
		}
	}
}

func ForwardResponse(w http.ResponseWriter, r *http.Response) {
	CopyHeaders(w, r)
	w.WriteHeader(r.StatusCode)
	// Errors frequently occur writing the body when the client hangs up.
	io.Copy(w, r.Body)
	r.Body.Close()
}

func SetOriginRequestForwardingHeaders(o, f *http.Request) {
	xff := o.Header.Get("X-Forwarded-For")
	hop, _, _ := net.SplitHostPort(f.RemoteAddr)
	if xff == "" {
		xff = hop
	} else {
		xff += "," + hop
	}
	o.Header.Set("X-Forwarded-For", xff)
	o.Header.Set("X-Forwarded-Proto", OriginatingProtocol(f))
}

// w is for the client response. r is the request to send to the origin
// (already "forwarded"). originUrl is where to send the request.
func ReverseProxyUpgrade(w http.ResponseWriter, r *http.Request, originUrl string) (err error) {
	u, err := url.Parse(originUrl)
	if err != nil {
		return
	}
	oc, err := net.Dial("tcp", u.Host)
	if err != nil {
		return
	}
	defer oc.Close()
	err = r.Write(oc)
	if err != nil {
		return
	}
	originConnReadBuffer := bufio.NewReader(oc)
	originResp, err := http.ReadResponse(originConnReadBuffer, r)
	if err != nil {
		return
	}
	if originResp.StatusCode != 101 {
		ForwardResponse(w, originResp)
		return
	}
	cc, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}
	defer cc.Close()
	originResp.Write(cc)
	go io.Copy(oc, cc)
	// Let the origin connection control when this routine returns, as we
	// should trust it more.
	io.Copy(cc, originConnReadBuffer)
	return
}

func ReverseProxy(w http.ResponseWriter, r *http.Request, originUrl string, client *http.Client) (err error) {
	originRequest, err := RedirectedRequest(r, originUrl)
	if err != nil {
		return
	}
	SetOriginRequestForwardingHeaders(originRequest, r)
	if r.Header.Get("Connection") == "Upgrade" {
		return ReverseProxyUpgrade(w, originRequest, originUrl)
	}
	rt := client.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	originResp, err := rt.RoundTrip(originRequest)
	if err != nil {
		return
	}
	ForwardResponse(w, originResp)
	return
}
