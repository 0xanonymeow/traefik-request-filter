// Package traefik_request_filter Filter your Traefik request headers, query parameters, and body.
package traefik_request_filter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

type body struct {
	Regex string
	JSON  map[string]interface{}
}

// Config the plugin configuration.
type Config struct {
	Headers map[string]string
	Query   map[string]string
	Body    body
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		Headers: make(map[string]string),
		Body: body{
			Regex: "",
			JSON:  make(map[string]interface{}),
		},
		Query: make(map[string]string),
	}
}

// RequestFilter a traefik_request_filter plugin.
type RequestFilter struct {
	next    http.Handler
	name    string
	headers map[string]string
	query   map[string]string
	body    body
}

// New created a new request filter plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	fmt.Printf("Creating plugin: %s instance: %+v, ctx: %+v\n", name, *config, ctx)

	return &RequestFilter{
		next:    next,
		name:    name,
		headers: config.Headers,
		query:   config.Query,
		body:    config.Body,
	}, nil
}

func (rf *RequestFilter) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if (rf.body.Regex != "" || len(rf.body.JSON) != 0) && req.Method == http.MethodTrace {
		fmt.Println("TRACE request is forbidden")
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	if isForbidden := filterHeaders(rw, rf.headers, req.Header); isForbidden {
		return
	}

	if isForbidden := filterQuery(rw, rf.query, req.URL.Query()); isForbidden {
		return
	}

	if req.ContentLength != 0 {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			fmt.Println("Failed to read request body")
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		isForbidden := filterBody(rw, rf.body, req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		if isForbidden {
			return
		}
	}

	rf.next.ServeHTTP(rw, req)
}

func filterHeaders(rw http.ResponseWriter, headers map[string]string, rheaders http.Header) bool {
	for hkey, hvalue := range headers {
		for rkey, rvalues := range rheaders {
			if hkey != rkey {
				continue
			}

			return compareWithDelimiters(rw, rkey, hvalue, rvalues)
		}
	}

	return false
}

func filterQuery(rw http.ResponseWriter, query map[string]string, rquery url.Values) bool {
	for qkey, qvalue := range query {
		for rkey, rvalues := range rquery {
			if qkey != rkey {
				continue
			}

			return compareWithDelimiters(rw, rkey, qvalue, rvalues)
		}
	}

	return false
}

func filterBody(rw http.ResponseWriter, body body, rbody io.ReadCloser) bool {
	// TODO: regex
	// if body.Regex != "" {
	// }

	if len(body.JSON) == 0 {
		return false
	}

	var rdata map[string]interface{}

	decoder := json.NewDecoder(rbody)
	err := decoder.Decode(&rdata)
	if err != nil {
		fmt.Println("Failed to decode request body")
		rw.WriteHeader(http.StatusBadRequest)
		return false
	}

	for bkey, bvalue := range body.JSON {
		for rkey, rvalue := range rdata {
			if bkey != rkey {
				continue
			}

			return compareBody(rw, rkey, bvalue, rvalue)
		}
	}

	return false
}

func compareWithDelimiters(rw http.ResponseWriter, rkey, value string, rvalues []string) bool {
	slice := strings.FieldsFunc(value, splitter)

	for _, value := range slice {
		for _, rvalue := range rvalues {
			if value == rvalue {
				forbid(rw, rkey, rvalue)
				return true
			}
		}
	}

	return false
}

func compareBody(rw http.ResponseWriter, rkey string, bvalue, rvalue interface{}) bool {
	bvaluebool, isBvalueBool := isBool(bvalue)
	bvaluefloat64, isBvalueFloat64 := isFloat64(bvalue)

	switch {
	case isBvalueBool && bvaluebool == rvalue:
		forbid(rw, rkey, rvalue)
		return true
	case isBvalueFloat64 && bvaluefloat64 == rvalue:
		forbid(rw, rkey, rvalue)
		return true
	case isSlice(bvalue):
		sbvalue := reflectTypeSlice(bvalue)
		return compareBodySlice(rw, rkey, sbvalue, rvalue)
	case bvalue == rvalue:
		forbid(rw, rkey, rvalue)
		return true
	}

	return false
}

func compareBodySlice(rw http.ResponseWriter, rkey string, sbvalue []interface{}, rvalue interface{}) bool {
	if isSlice(rvalue) {
		for _, bv := range sbvalue {
			//nolint:forcetypeassert
			for _, rv := range rvalue.([]interface{}) {
				if bv == rv {
					forbid(rw, rkey, rv)
					return true
				}
			}
		}
	}

	for _, bv := range sbvalue {
		if bv == rvalue {
			forbid(rw, rkey, rvalue)
			return true
		}
	}

	return false
}

func reflectTypeSlice(bvalue interface{}) []interface{} {
	s := []interface{}{}

	//nolint:forcetypeassert
	for _, bv := range bvalue.([]interface{}) {
		bvbool, isBvBool := isBool(bv)
		bvfloat64, isBvFloat64 := isFloat64(bv)

		switch {
		case isBvBool:
			s = append(s, bvbool)
		case isBvFloat64:
			s = append(s, bvfloat64)
		default:
			s = append(s, bv)
		}
	}

	return s
}

func forbid(rw http.ResponseWriter, rkey, value interface{}) {
	fmt.Printf("(key: %s, value: %s) is forbidden", rkey, value)
	rw.WriteHeader(http.StatusForbidden)
}

func isBool(v interface{}) (bool, bool) {
	vstr, ok := v.(string)

	if !ok {
		return false, false
	}

	b, err := strconv.ParseBool(vstr)
	if err != nil {
		return false, false
	}

	return b, true
}

func isFloat64(v interface{}) (float64, bool) {
	vstr, ok := v.(string)

	if !ok {
		return 0, false
	}

	f, err := strconv.ParseFloat(vstr, 64)
	if err != nil {
		return 0, false
	}

	return f, true
}

func isSlice(v interface{}) bool {
	return reflect.TypeOf(v).Kind() == reflect.Slice
}

func splitter(r rune) bool {
	return r == ',' || r == ';'
}
