package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/jrwren/ugly_brysen/eater"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const (
	authcookiename = "blah"
	cachepath      = "./cache"
)

/* goal: return this data for any quote:
Apple Inc. (AAPL) -> 168.82 (+2.10 +1.26%) |
52w L/H 104.08/169.65 | P/E: 19.17 |
Div/yield: 2.52/1.55

P/S would be a nice bonus.
*/

// Quote is a quote.
type Quote struct {
	Symbol           string   `json:"symbol"`
	Name             string   `json:"name"`
	Price            mfloat64 `json:"price"`
	Change           mfloat64 `json:"change"`
	ChangePct        mfloat64 `json:"changepct"`
	DayHigh          mfloat64 `json:"day_high,omitempty"`
	DayLow           mfloat64 `json:"day_low,omitemptyw"`
	FiftyTwoWeekHigh mfloat64 `json:"fifty_two_week_high,omitemptyh"`
	FiftyTwoWeekLow  mfloat64 `json:"fifty_two_week_low,omitemptyw"`
	PE               mfloat64 `json:"pe,omitempty"`
	PB               mfloat64 `json:"pb,omitempty"`
	PS               mfloat64 `json:"ps,omitemptys"`
	Div              mfloat64 `json:"div,omitempty"`
	Yield            mfloat64 `json:"yield,omitempty"`
}

// User will be a GH user when I make oauth2 GH work.
type User struct {
	Name     string
	Email    string
	Password string
	Picture  string
}

var (
	sessions map[string]User
	mu       sync.Mutex
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	sessions = make(map[string]User)
	//loadSessions()
	ctx := context.Background()
	conf := &oauth2.Config{
		ClientID:     "",
		ClientSecret: "-L",
		RedirectURL:  "",
		Scopes: []string{
			"email",
			"openid",
		},
		Endpoint: github.Endpoint,
	}
	url := conf.AuthCodeURL("state")
	root := func(w http.ResponseWriter, r *http.Request) {
		ok := false
		var user User
		blah, err := r.Cookie(authcookiename)
		if err != nil {
			log.Print("no auth cookie")
			goto nope
		}
		user, ok = sessions[blah.Value]
	nope:
		fmt.Fprintf(w, `
<html>
	<head></head>
	<body>`)
		defer fmt.Fprintf(w, `</body>
		</html>`)
		if !ok {
			fmt.Fprintf(w, `
		<a href="%s">Click here to login with github *** not working yet</a>`, url)
			return
		}
		fmt.Fprintf(w, `<div>Hello and welcome.</div>`)
		fmt.Fprintf(w, "<div>your email is %s and your name is %s",
			user.Email, user.Name)

	}
	r := http.NewServeMux()
	r.HandleFunc("/", root)
	r.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		tok, err := conf.Exchange(ctx, code)
		if err != nil {
			fmt.Fprintf(w, "Something when wrong. Maybe that code was already used. Backup and try again.")
			log.Print(err, code)
			return
		}
		jwt := tok.Extra("id_token").(string)
		jwtp := strings.SplitN(jwt, ".", 3)
		idt, err := base64.StdEncoding.DecodeString(jwtp[1])
		if err != nil {
			log.Fatal(err, tok.Extra("id_token"))
		}
		var token map[string]interface{}
		err = json.Unmarshal(idt, &token)
		if err != nil {
			log.Fatal(err, string(idt))
		}
		fmt.Println(token)
		val := rand.Uint64()
		cookie := &http.Cookie{
			Name:  authcookiename,
			Value: strconv.FormatUint(val, 36),
			//Secure:   true,
			//HttpOnly: true,
		}
		fmt.Printf("got code:%s\n", code)
		name, _ := token["name"].(string)
		picture, _ := token["picture"].(string)
		mu.Lock()
		sessions[cookie.Value] = User{
			Name:    name,
			Email:   token["email"].(string),
			Picture: picture,
		}
		sf, err := os.Create("sessions")
		if err != nil {
			log.Print(err)
		}
		defer sf.Close()
		json.NewEncoder(sf).Encode(sessions)
		mu.Unlock()
		log.Print("setting cookie ", cookie)
		http.SetCookie(w, cookie)
		http.Redirect(w, r, "/", http.StatusFound)
		fmt.Fprintf(w, "success, redirecting to main page")
	})
	r.HandleFunc("/quote", quote)
	log.Fatal(http.ListenAndServe(":8081", t(
		handlers.CombinedLoggingHandler(os.Stdout,
			handlers.CompressHandler(r)))))
}

func t(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.RemoteAddr, "204.44.116.103") {
			http.Error(w, fmt.Sprintf("nope"), http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func loadSessions() {
	sf, err := os.Open("sessions")
	if err != nil {
		log.Print(err)
	}
	defer sf.Close()
	err = json.NewDecoder(sf).Decode(&sessions)
	if err != nil {
		log.Print(err)
	}
}

func quote(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	name = strings.TrimSpace(name)
	var rdoc io.Reader
	fetch := false
	cf, err := os.Open(path.Join(cachepath, strings.ToLower(name)))
	if err != nil {
		fetch = true
	}
	defer cf.Close()
	if cf != nil {
		st, err := cf.Stat()
		if err != nil {
			log.Print(err)
		}
		exptime := time.Now().Add(-5 * time.Minute)
		if st.ModTime().Before(exptime) {
			fetch = true
		}
	}
	if fetch {
		rget, err := http.Get("https://finance.yahoo.com/quote/" + name)
		if err != nil {
			fmt.Println("ERROR ", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		defer rget.Body.Close()

		ct, err := os.Create(path.Join(cachepath, strings.ToLower(name)))
		if err != nil {
			log.Print(err)
		}
		defer ct.Close()
		rdoc = io.TeeReader(rget.Body, ct)
	} else {
		rdoc = cf
	}
	O_O, err := ioutil.ReadAll(rdoc)
	if err != nil {
		log.Print(err)
	}
	o_o := eater.ExtractJSONString(string(O_O), "root.App.main")
	o_o = strings.TrimRight(o_o, ";")
	d := make(map[string]map[string]map[string]map[string]interface{})

	err = json.Unmarshal([]byte(o_o), &d)
	/*
		if err != nil {
			log.Printf("%v: doc: %s\n", err, o_o[0:minsl(10, o_o)])
		}*/
	// .context.dispatcher.stores.PageStore.pageData
	// .context.dispatcher.stores.QuoteSummaryStore
	stores := d["context"]["dispatcher"]["stores"]
	respjson := make(map[string]interface{})
	pagestore, ok := stores["PageStore"].(map[string]interface{})
	if !ok {
		log.Printf("pagestore not map[string]interface{} %#v\nstores:%#v\n", stores["PageStore"], stores)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	respjson["otherjunk"] = pagestore["pageData"]
	respjson["quote"] = stores["QuoteSummaryStore"]
	if strings.Contains(r.Header.Get("Accept"), "json") && r.URL.Query().Get("full") == "true" {
		err = json.NewEncoder(w).Encode(respjson)
		if err != nil {
			log.Print(err)
		}
		return
	}
	quote, ok := stores["QuoteSummaryStore"].(map[string]interface{})
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	pd := pagestore["pageData"].(map[string]interface{})
	title := pd["title"].(string)
	summaryi := strings.Index(title, "Summary for")
	title = title[summaryi+12:]
	dashY := strings.Index(title, " - Yahoo Finance")
	fullname := title[:dashY]

	price := quote["price"].(map[string]interface{})
	defaultKeyStats := quote["defaultKeyStatistics"].(map[string]interface{}) // has forwardPE, enterpriseValue
	pb := getrawfloat(defaultKeyStats["priceToBook"])

	summaryDetail := quote["summaryDetail"].(map[string]interface{})
	ps := getrawfloat(summaryDetail["priceToSalesTrailing12Months"])
	if math.IsNaN(float64(ps)) {
		ps = getrawfloat(defaultKeyStats["priceToSalesTrailing12Months"])
	}

	financialData := quote["financialData"].(map[string]interface{})
	cprice := getrawfloat(financialData["currentPrice"])

	change := getrawfloat(price["regularMarketChange"])
	changepct := getrawfloat(price["regularMarketChangePercent"]) * 100
	yrhi := getrawfloat(summaryDetail["fiftyTwoWeekHigh"])
	yrlo := getrawfloat(summaryDetail["fiftyTwoWeekLow"])
	dayhi := getrawfloat(price["regularMarketDayHigh"])
	daylo := getrawfloat(price["regularMarketDayLow"])
	pe := getrawfloat(summaryDetail["forwardPE"])
	div := getrawfloat(summaryDetail["dividendRate"])
	yield := getrawfloat(summaryDetail["dividendYield"]) * 100

	if strings.Contains(r.Header.Get("Accept"), "json") {
		q := Quote{
			Name:             fullname,
			Symbol:           name,
			Price:            cprice,
			Change:           change,
			ChangePct:        changepct,
			DayHigh:          dayhi,
			DayLow:           daylo,
			FiftyTwoWeekHigh: yrhi,
			FiftyTwoWeekLow:  yrlo,
			PE:               pe,
			PB:               pb,
			PS:               ps,
			Div:              div,
			Yield:            yield,
		}
		err = json.NewEncoder(w).Encode(q)
		if err != nil {
			log.Print(err)
		}
		return
	}
	title = fmt.Sprintf("%s (%s) -> %.2f (%+.2f %+.2f%%) | Day L/H %v/%v | 52w L/H %v/%v | P/E: %v | P/S: %v | P/B %v | Div/yield %v/%v",
		fullname, strings.ToUpper(name), cprice, change, changepct, dayhi, daylo,
		yrhi, yrlo, pe,
		ps, pb, div, yield)
	body := "<h1>hello</h1>\n" + title
	htmlResponseWithTitle(w, title, body)
}

func getrawfloat(i interface{}) mfloat64 {
	//return i.(map[string]interface{})["raw"].(float64)
	m, ok := i.(map[string]interface{})
	if !ok {
		log.Printf("could not convert %#v to raw val 1\n", i)
		return mfloat64(math.NaN())
	}
	r, ok := m["raw"]
	if !ok {
		// I guess this is pretty common
		//log.Printf("could not convert %#v to raw val 2\n", i)
		return mfloat64(math.NaN())
	}
	f, ok := r.(float64)
	if !ok {
		log.Printf("could not convert %#v to raw val 3\n", i)
		return mfloat64(math.NaN())
	}
	return mfloat64(f)
}

type mfloat64 float64

func (f mfloat64) String() string {
	if math.IsNaN(float64(f)) {
		return "-"
	}
	return fmt.Sprintf("%.2f", f)
}

func (v mfloat64) MarshalJSON() ([]byte, error) {
	type optional struct {
	}
	if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
		return []byte("{}"), nil
	}
	return []byte(fmt.Sprintf("{\"value\": %f }", v)), nil
}

func htmlResponseWithTitle(w http.ResponseWriter, title, body string) {
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>%s</title><meta name="note" content="add http header 'Accept: json' for json response"></head>
	<body>%s</body>
</html>`, title, body)
}

// minsl min string len
func minsl(i int, s string) int {
	if len(s) < i {
		return len(s)
	}
	return i
}
