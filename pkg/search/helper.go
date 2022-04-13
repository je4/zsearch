// Package search /*
package search

import (
	"crypto/md5"
	"fmt"
	"github.com/golang-jwt/jwt"
	"github.com/golang/snappy"
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/net/idna"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var _logformat = logging.MustStringFormatter(
	`%{time:2006-01-02T15:04:05.000} %{module}::%{shortfunc} [%{shortfile}] > %{level:.5s} - %{message}`,
)

var bearerPrefix = "Bearer "

func Min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func TrimLength(str string, length int, suffix string) string {
	if len(str) <= length {
		return str
	}
	runes := []rune(str)
	return string(runes[0:length-len(suffix)]) + suffix
}

func Hash(data interface{}) ([16]byte, error) {
	d, err := bson.Marshal(data)
	if err != nil {
		return [16]byte{}, errors.Wrap(err, "cannot marshal data")
	}
	return md5.Sum(d), nil
}

func Compress(data []byte) []byte {
	return snappy.Encode(nil, data)
}

func Decompress(data []byte) ([]byte, error) {
	return snappy.Decode(nil, data)
}

func UrlAmp(u string, ampCache string, t string) (string, error) {
	url, err := url.Parse(u)
	if err != nil {
		return "", errors.Wrapf(err, "cannot parse external address %s", u)
	}
	// convert domain to unicode
	domain, err := idna.ToUnicode(url.Host)
	if err != nil {
		return "", errors.Wrapf(err, "cannot convert domain %s from punycode to unicode", url.Host)
	}
	// replace all - with --
	domain = strings.ReplaceAll(domain, "-", "--")
	// replace . with -
	domain = strings.ReplaceAll(domain, ".", "-")

	return strings.TrimRight(fmt.Sprintf("https://%s.%s/%s/s/%s/%s", domain, ampCache, t, url.Host, strings.Trim(url.Path, "/")), "/"), nil
}

func AppendIfMissing(slice []string, s string) []string {
	for _, ele := range slice {
		if ele == s {
			return slice
		}
	}
	return append(slice, s)
}

func InList(slice []string, s string) bool {
	for _, ele := range slice {
		if ele == s {
			return true
		}
	}
	return false
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func GetClaim(claim map[string]interface{}, name string) (string, error) {
	val, ok := claim[name]
	if !ok {
		return "", fmt.Errorf("no claim %s found", name)
	}
	valstr, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("claim %s not a string", name)
	}
	return valstr, nil
}

func buildMatcher(reg *regexp.Regexp) func(r *http.Request, rm *mux.RouteMatch) bool {
	return func(r *http.Request, rm *mux.RouteMatch) bool {
		matches := reg.FindStringSubmatch(r.URL.Path)
		if matches == nil {
			return false
		}
		rm.Vars = make(map[string]string)
		for i, name := range reg.SubexpNames() {
			if i != 0 && name != "" {
				rm.Vars[name] = matches[i]
			}
		}
		return true
	}
}

func CheckRequestJWT(req *http.Request, secret string, alg []string, subject string) error {
	var token []string
	var ok bool

	// first check Bearer token
	reqToken := req.Header.Get("Authorization")
	n := len(bearerPrefix)
	if len(reqToken) > n && reqToken[:n] == bearerPrefix {
		token = []string{reqToken[n:]}
	}
	// no bearer --> check token parameter
	if len(token) < 1 {
		query := req.URL.Query()
		token, ok = query["token"]
		// sometimes auth is used instead of token...
		if !ok {
			token, _ = query["auth"]
		}
		// no bearer, no token, no auth...
		if len(token) < 1 {
			return errors.New(fmt.Sprintf("Access denied: no jwt token found"))
		}
	}
	if err := CheckJWT(token[0], secret, alg, subject); err != nil {
		return errors.Wrapf(err, "Access denied: token check failed")
	}
	return nil
}

func CheckJWTValid(tokenstring string, secret string, alg []string) (map[string]interface{}, error) {
	token, err := jwt.Parse(tokenstring, func(token *jwt.Token) (interface{}, error) {
		talg := token.Method.Alg()
		algOK := false
		for _, a := range alg {
			if talg == a {
				algOK = true
				break
			}
		}
		if !algOK {
			return false, fmt.Errorf("unexpected signing method (allowed are %v): %v", alg, token.Header["alg"])
		}

		return []byte(secret), nil
	})
	if err != nil {
		return map[string]interface{}{}, fmt.Errorf("invalid token: %v", err)
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if !ok {
			return map[string]interface{}{}, fmt.Errorf("Cannot get claims from token %s", tokenstring)
		}
		return claims, nil
	}
	return map[string]interface{}{}, fmt.Errorf("Token %s not valid", tokenstring)
}

func CheckJWT(tokenstring string, secret string, alg []string, subject string) error {
	subject = strings.TrimRight(strings.ToLower(subject), "/")
	claims, err := CheckJWTValid(tokenstring, secret, alg)
	if err != nil {
		return errors.Wrap(err, "invalid token")
	}
	sub, ok := claims["sub"]
	if !ok {
		return fmt.Errorf("no sub claim in %s", tokenstring)
	}
	substr, ok := sub.(string)
	if !ok {
		return fmt.Errorf("sub claim %v not a string in %s", sub, tokenstring)
	}
	if strings.ToLower(substr) != subject {
		return fmt.Errorf("Invalid subject [%s]. Should be [%s] in %s", substr, subject, tokenstring)
	}
	return nil
}

func CreateLogger(module string, logfile string, loglevel string) (log *logging.Logger, lf *os.File) {
	log = logging.MustGetLogger(module)
	var err error
	if logfile != "" {
		lf, err = os.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Errorf("Cannot open logfile %v: %v", logfile, err)
		}
		//defer lf.CloseInternal()

	} else {
		lf = os.Stderr
	}
	backend := logging.NewLogBackend(lf, "", 0)
	backendLeveled := logging.AddModuleLevel(backend)
	backendLeveled.SetLevel(logging.GetLevel(loglevel), "")

	logging.SetFormatter(_logformat)
	logging.SetBackend(backendLeveled)

	return
}

func NewJWT(secret string, subject string, alg string, valid int64, domain string, issuer string, userId string) (tokenString string, err error) {

	var signingMethod jwt.SigningMethod
	switch strings.ToLower(alg) {
	case "hs256":
		signingMethod = jwt.SigningMethodHS256
	case "hs384":
		signingMethod = jwt.SigningMethodHS384
	case "hs512":
		signingMethod = jwt.SigningMethodHS512
	case "es256":
		signingMethod = jwt.SigningMethodES256
	case "es384":
		signingMethod = jwt.SigningMethodES384
	case "es512":
		signingMethod = jwt.SigningMethodES512
	case "ps256":
		signingMethod = jwt.SigningMethodPS256
	case "ps384":
		signingMethod = jwt.SigningMethodPS384
	case "ps512":
		signingMethod = jwt.SigningMethodPS512
	default:
		return "", errors.Wrapf(err, "invalid signing method %s", alg)
	}
	exp := time.Now().Unix() + valid
	claims := jwt.MapClaims{
		"sub": strings.ToLower(subject),
		"exp": exp,
	}
	// keep jwt short, no empty Fields
	if domain != "" {
		claims["aud"] = domain
	}
	if issuer != "" {
		claims["iss"] = issuer
	}
	if userId != "" {
		claims["user"] = userId
	}

	token := jwt.NewWithClaims(signingMethod, claims)
	//	log.Println("NewJWT( ", secret, ", ", subject, ", ", exp)
	tokenString, err = token.SignedString([]byte(secret))
	return tokenString, err
}

func SingleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
