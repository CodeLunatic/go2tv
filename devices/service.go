package devices

import (
	"net/http"
	"regexp"
	"strconv"
)

// 从ssdp包复制出来的代码
// Service is discovered service.
type Service struct {

	// Receive from ip:port
	From string

	// Type is a property of "ST"
	Type string

	// USN is a property of "USN"
	USN string

	// Location is a property of "LOCATION"
	Location string

	// Server is a property of "SERVER"
	Server string

	rawHeader http.Header
	maxAge    *int
}

var rxMaxAge = regexp.MustCompile(`\bmax-age\s*=\s*(\d+)\b`)

func extractMaxAge(s string, value int) int {
	v := value
	if m := rxMaxAge.FindStringSubmatch(s); m != nil {
		i64, err := strconv.ParseInt(m[1], 10, 32)
		if err == nil {
			v = int(i64)
		}
	}
	return v
}

// MaxAge extracts "max-age" value from "CACHE-CONTROL" property.
func (s *Service) MaxAge() int {
	if s.maxAge == nil {
		s.maxAge = new(int)
		*s.maxAge = extractMaxAge(s.rawHeader.Get("CACHE-CONTROL"), -1)
	}
	return *s.maxAge
}

// Header returns all properties in response of search.
func (s *Service) Header() http.Header {
	return s.rawHeader
}
