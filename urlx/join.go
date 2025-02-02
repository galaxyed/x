package urlx

import (
	"net/url"
	"path"

	"github.com/galaxyed/x/cmdx"
)

// MustJoin joins the paths of two URLs. Fatals if first is not a DSN.
func MustJoin(first string, parts ...string) string {
	u, err := url.Parse(first)
	if err != nil {
		cmdx.Fatalf("Unable to parse %s: %s", first, err)
	}
	return AppendPaths(u, parts...).String()
}

// AppendPaths appends the provided paths to the url.
func AppendPaths(u *url.URL, paths ...string) (ep *url.URL) {
	ep = Copy(u)
	if len(paths) == 0 {
		return ep
	}

	ep.Path = path.Join(append([]string{ep.Path}, paths...)...)

	last := paths[len(paths)-1]
	if last[len(last)-1] == '/' {
		ep.Path = ep.Path + "/"
	}

	return ep
}

// SetQuery appends the provided url values to the DSN's query string.
func SetQuery(u *url.URL, query url.Values) (ep *url.URL) {
	ep = Copy(u)
	q := ep.Query()

	for k := range query {
		q.Set(k, query.Get(k))
	}

	ep.RawQuery = q.Encode()
	return ep
}
