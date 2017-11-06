package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"io"

	"path"

	"github.com/PuerkitoBio/goquery"
	"github.com/jrwren/ugly_brysen/eater"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const (
	authcookiename = "blah"
	cachepath      = "./cache"
)

/* goal: return this data for any quote:
/Apple Inc. (AAPL) -> 168.82 (+2.10 +1.26%) |
52w L/H 104.08/169.65 | P/E: 19.17 |
Div/yield: 2.52/1.55

P/S would be a nice bonus.
*/

// Quote is a quote.
type Quote struct {
	Symbol    string
	Name      string
	Price     float64
	Change    float64
	PctChange float64
	_52wkHigh float64
	_52wkLow  float64
	PE        float64
	Div       float64
	Yield     float64
	PS        float64
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
	sessions = make(map[string]User)
	loadSessions()
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
	http.HandleFunc("/", root)
	http.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
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
	http.HandleFunc("/quote", quote)
	log.Fatal(http.ListenAndServe(":8081", nil))
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

func etradequote(w http.ResponseWriter, r *http.Request) {
	log.Print(r)
	name := r.URL.Query().Get("name")
	name = strings.TrimSpace(name)
	resp, err := http.Get("https://etws.etrade.com/market/rest/quote/" + name + ".json?detailFlag=FUNDAMENTAL")
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Println("ERROR ", err, resp.StatusCode)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	fmt.Printf("%#v\n", resp)
}

func quote(w http.ResponseWriter, r *http.Request) {
	log.Print(r)
	name := r.URL.Query().Get("name")
	name = strings.TrimSpace(name)
	var rdoc io.Reader
	cf, err := os.Open(path.Join(cachepath, name))
	if err != nil {
		log.Print(err)
		rget, err := http.Get("https://finance.yahoo.com/quote/" + name)
		if err != nil {
			fmt.Println("ERROR ", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		defer rget.Body.Close()

		ct, err := os.Create(path.Join(cachepath, name))
		if err != nil {
			log.Print(err)
		}
		defer ct.Close()
		rdoc = io.TeeReader(rget.Body, ct)
	}
	defer cf.Close()
	if cf != nil {
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
	if err != nil {
		log.Print(err)
	}
	// .context.dispatcher.stores.PageStore.pageData
	// .context.dispatcher.stores.QuoteSummaryStore
	stores := d["context"]["dispatcher"]["stores"]
	respjson := make(map[string]interface{})
	ps, ok := stores["PageStore"].(map[string]interface{})
	if !ok {
		log.Print("pagestore not map[string]interface{}")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	respjson["otherjunk"] = ps["pageData"]
	respjson["quote"] = stores["QuoteSummaryStore"]
	err = json.NewEncoder(w).Encode(respjson)
	if err != nil {
		log.Print(err)
	}
}

func wTFquery(rdoc io.Reader, name string, w http.ResponseWriter) {
	// doc, err := goquery.NewDocument("https://finance.yahoo.com/quote/" + name)
	doc, err := goquery.NewDocumentFromReader(rdoc)
	if err != nil {
		fmt.Println("ERROR ", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	fmt.Printf("%#v", doc)
	// //*[@id="quote-header-info"]/div[3]/div[1]/div/span[1]/text()
	qe := doc.Find(`#quote-header-info > div.Mt\(6px\).smartphone_Mt\(15px\) > div.D\(ib\).Maw\(65\%\).Maw\(70\%\)--tab768.Ov\(h\) > div > span.Trsdu\(0\.3s\).Fw\(b\).Fz\(36px\).Mb\(-4px\).D\(ib\)`).First()
	p, err := strconv.ParseFloat(qe.Text(), 64)
	if err != nil {
		fmt.Println("ERROR ", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	cop := doc.Find(`#quote-header-info > div.Mt\(15px\) > div.D\(ib\).Mt\(-5px\).Mend\(20px\).Maw\(56\%\)--tab768.Maw\(52\%\).Ov\(h\).smartphone_Maw\(85\%\).smartphone_Mend\(10px\) > div.D\(ib\) > h1`).First()
	co := cop.Text()
	q := &Quote{
		Symbol: name,
		Name:   co,
		Price:  p,
	}
	json.NewEncoder(w).Encode(q)
}
