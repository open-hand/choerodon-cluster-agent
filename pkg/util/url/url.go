package url

import (
	"fmt"
	"net/url"
)

func ParseURL(base *url.URL, refStr string) (*url.URL, error) {
	var newRefStr string
	if base.RawQuery != "" {
		newRefStr = fmt.Sprintf("%s?%s", refStr, base.RawQuery)
	} else {
		newRefStr = refStr
	}
	return base.Parse(newRefStr)
}
