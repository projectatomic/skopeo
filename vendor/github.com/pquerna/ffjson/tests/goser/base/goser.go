/**
 *  Copyright 2014 Paul Querna
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package goser

import (
	"io"
	"net"
	"time"
)

// CacheStatus of goser
type CacheStatus int32

const (
	// CACHESTATUSUNKNOWN unknown cache status
	CACHESTATUSUNKNOWN CacheStatus = 0
	// CACHESTATUSMISS miss cache status
	CACHESTATUSMISS CacheStatus = 1
	// CACHESTATUSEXPIRED exipred cache status
	CACHESTATUSEXPIRED CacheStatus = 2
	// CACHESTATUSHIT hit cache status
	CACHESTATUSHIT CacheStatus = 3
)

// HTTPProtocol of goser
type HTTPProtocol int32

const (
	// HTTPPROTOCOLUNKNOWN http protocol unknown
	HTTPPROTOCOLUNKNOWN HTTPProtocol = 0
	// HTTPPROTOCOL10 http protocol 10
	HTTPPROTOCOL10 HTTPProtocol = 1
	// HTTPPROTOCOL11 http protocol 11
	HTTPPROTOCOL11 HTTPProtocol = 2
)

// HTTPMethod of goser
type HTTPMethod int32

const (
	// HTTPMETHODUNKNOWN unknown http method
	HTTPMETHODUNKNOWN HTTPMethod = 0
	// HTTPMETHODGET get http method
	HTTPMETHODGET HTTPMethod = 1
	// HTTPMETHODPOST post http method
	HTTPMETHODPOST HTTPMethod = 2
	// HTTPMETHODDELETE delete http method
	HTTPMETHODDELETE HTTPMethod = 3
	// HTTPMETHODPUT put http method
	HTTPMETHODPUT HTTPMethod = 4
	// HTTPMETHODHEAD head http method
	HTTPMETHODHEAD HTTPMethod = 5
	// HTTPMETHODPURGE purge http method
	HTTPMETHODPURGE HTTPMethod = 6
	// HTTPMETHODOPTIONS options http method
	HTTPMETHODOPTIONS HTTPMethod = 7
	// HTTPMETHODPROPFIND propfind http method
	HTTPMETHODPROPFIND HTTPMethod = 8
	// HTTPMETHODMKCOL mkcol http method
	HTTPMETHODMKCOL HTTPMethod = 9
	// HTTPMETHODPATCH patch http method
	HTTPMETHODPATCH HTTPMethod = 10
)

// OriginProtocol type
type OriginProtocol int32

const (
	// ORIGINPROTOCOLUNKNOWN origin protocol unknown
	ORIGINPROTOCOLUNKNOWN OriginProtocol = 0
	// ORIGINPROTOCOLHTTP origin protocol http
	ORIGINPROTOCOLHTTP OriginProtocol = 1
	// ORIGINPROTOCOLHTTPS origin protocol https
	ORIGINPROTOCOLHTTPS OriginProtocol = 2
)

// HTTP struct type
type HTTP struct {
	Protocol     HTTPProtocol `json:"protocol"`
	Status       uint32       `json:"status"`
	HostStatus   uint32       `json:"hostStatus"`
	UpStatus     uint32       `json:"upStatus"`
	Method       HTTPMethod   `json:"method"`
	ContentType  string       `json:"contentType"`
	UserAgent    string       `json:"userAgent"`
	Referer      string       `json:"referer"`
	RequestURI   string       `json:"requestURI"`
	Unrecognized []byte       `json:"-"`
}

// Origin struct
type Origin struct {
	IP       IP             `json:"ip"`
	Port     uint32         `json:"port"`
	Hostname string         `json:"hostname"`
	Protocol OriginProtocol `json:"protocol"`
}

// ZonePlan type
type ZonePlan int32

const (
	// ZONEPLANUNKNOWN unknwon zone plan
	ZONEPLANUNKNOWN ZonePlan = 0
	// ZONEPLANFREE free zone plan
	ZONEPLANFREE ZonePlan = 1
	// ZONEPLANPRO pro zone plan
	ZONEPLANPRO ZonePlan = 2
	// ZONEPLANBIZ biz zone plan
	ZONEPLANBIZ ZonePlan = 3
	// ZONEPLANENT ent zone plan
	ZONEPLANENT ZonePlan = 4
)

// Country type
type Country int32

const (
	// COUNTRYUNKNOWN unknwon country
	COUNTRYUNKNOWN Country = 0
	// COUNTRYUS us country
	COUNTRYUS Country = 238
)

// Log struct
type Log struct {
	Timestamp    int64       `json:"timestamp"`
	ZoneID       uint32      `json:"zoneId"`
	ZonePlan     ZonePlan    `json:"zonePlan"`
	HTTP         HTTP        `json:"http"`
	Origin       Origin      `json:"origin"`
	Country      Country     `json:"country"`
	CacheStatus  CacheStatus `json:"cacheStatus"`
	ServerIP     IP          `json:"serverIp"`
	ServerName   string      `json:"serverName"`
	RemoteIP     IP          `json:"remoteIp"`
	BytesDlv     uint64      `json:"bytesDlv"`
	RayID        string      `json:"rayId"`
	Unrecognized []byte      `json:"-"`
}

// IP type
type IP net.IP

// MarshalJSON function
func (ip IP) MarshalJSON() ([]byte, error) {
	return []byte("\"" + net.IP(ip).String() + "\""), nil
}

// UnmarshalJSON function
func (ip *IP) UnmarshalJSON(data []byte) error {
	if len(data) < 2 {
		return io.ErrShortBuffer
	}
	*ip = IP(net.ParseIP(string(data[1 : len(data)-1])).To4())
	return nil
}

const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/33.0.1750.146 Safari/537.36"

// NewLog creates a new log
func NewLog(record *Log) {
	record.Timestamp = time.Now().UnixNano()
	record.ZoneID = 123456
	record.ZonePlan = ZONEPLANFREE

	record.HTTP = HTTP{
		Protocol:    HTTPPROTOCOL11,
		Status:      200,
		HostStatus:  503,
		UpStatus:    520,
		Method:      HTTPMETHODGET,
		ContentType: "text/html",
		UserAgent:   userAgent,
		Referer:     "https://www.cloudflare.com/",
		RequestURI:  "/cdn-cgi/trace",
	}

	record.Origin = Origin{
		IP:       IP(net.IPv4(1, 2, 3, 4).To4()),
		Port:     8080,
		Hostname: "www.example.com",
		Protocol: ORIGINPROTOCOLHTTPS,
	}

	record.Country = COUNTRYUS
	record.CacheStatus = CACHESTATUSHIT
	record.ServerIP = IP(net.IPv4(192, 168, 1, 1).To4())
	record.ServerName = "metal.cloudflare.com"
	record.RemoteIP = IP(net.IPv4(10, 1, 2, 3).To4())
	record.BytesDlv = 123456
	record.RayID = "10c73629cce30078-LAX"
}
