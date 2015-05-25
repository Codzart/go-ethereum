package bzz

import (
	"fmt"
	"net/http"

	// "github.com/ethereum/go-ethereum/common/docserver"
	// "github.com/ethereum/go-ethereum/jsre"
)

type RoundTripper struct {
	Port string
}

func (self *RoundTripper) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	url := fmt.Sprintf("http://localhost:%s/%s/%s", self.Port, req.URL.Host, req.URL.Path)
	dpaLogger.Infof("roundtripper: proxying request '%s' to '%s'", req.RequestURI, url)
	return http.Get(url)
}
