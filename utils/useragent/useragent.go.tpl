package useragent

import (
	"crypto/md5"
	"encoding/binary"
	"os"
)

func hostnameToNumber() uint64 {
	hostname, err := os.Hostname()
	if err != nil {
		return 0
	}
	hash := md5.Sum([]byte(hostname))
	num := binary.BigEndian.Uint64(hash[:8])
	return num
}

var ua = []string{
	"Mozilla/5.0 (X11; Linux x86_64; rv:{{ .FirefoxRevision }}.0) Gecko/20100101 Firefox/{{ .FirefoxRevision }}.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X {{ .OSXVersion }}; rv:{{ .FirefoxRevision }}.0) Gecko/20100101 Firefox/{{ .FirefoxRevision }}.0",
	"Mozilla/5.0 (Windows NT {{ .NTVersion }}; Win64; x64; rv:{{ .FirefoxRevision }}.0) Gecko/20100101 Firefox/{{ .FirefoxRevision }}.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:{{ sub .FirefoxRevision 1 }}.0) Gecko/20100101 Firefox/{{ sub .FirefoxRevision 1 }}.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X {{ .OSXVersion }}; rv:{{ sub .FirefoxRevision 1 }}.0) Gecko/20100101 Firefox/{{ sub .FirefoxRevision 1 }}.0",
	"Mozilla/5.0 (Windows NT {{ .NTVersion }}; Win64; x64; rv:{{ sub .FirefoxRevision 1 }}.0) Gecko/20100101 Firefox/{{ sub .FirefoxRevision 1 }}.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:{{ sub .FirefoxRevision 2 }}.0) Gecko/20100101 Firefox/{{ sub .FirefoxRevision 2 }}.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X {{ .OSXVersion }}; rv:{{ sub .FirefoxRevision 2 }}.0) Gecko/20100101 Firefox/{{ sub .FirefoxRevision 2 }}.0",
	"Mozilla/5.0 (Windows NT {{ .NTVersion }}; Win64; x64; rv:{{ sub .FirefoxRevision 2 }}.0) Gecko/20100101 Firefox/{{ sub .FirefoxRevision 2 }}.0",
}

func Get() string {
	chosen := int(hostnameToNumber()) % len(ua)
	return ua[chosen]
}
