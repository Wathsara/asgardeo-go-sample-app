package main

import (
	"encoding/gob"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/pat"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/openidConnect"
)

var key = "_auth_session"
var store = sessions.NewCookieStore([]byte(key))
var sessionName = "auth"
var ASGARDEO_ORG_NAME = ""

func indexHandler(wr http.ResponseWriter, req *http.Request) {

	wr.WriteHeader(http.StatusOK)
	t, _ := template.ParseFiles("ui/index.html")
	t.Execute(wr, false)
}

func logout(wr http.ResponseWriter, req *http.Request) {

	err := gothic.Logout(wr, req)
	if err != nil {
		log.Print("Logout fail", err)
	}
	removeUserFromSession(wr, req)
	wr.Header().Set("Location", "/")
	wr.WriteHeader(http.StatusTemporaryRedirect)
}

func callback(wr http.ResponseWriter, req *http.Request) {

	user, err := gothic.CompleteUserAuth(wr, req)
	if err != nil {
		wr.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
		return
	}
	addUserToSession(wr, req, user)
	http.Redirect(wr, req, "/home", http.StatusFound)
}

func home(wr http.ResponseWriter, req *http.Request) {

	session, _ := store.Get(req, sessionName)
	user := session.Values["user"].(goth.User)
	if user.IDToken == "" {
		wr.Header().Set("Location", "/")
		wr.WriteHeader(http.StatusTemporaryRedirect)
		return
	}
	t, _ := template.ParseFiles("ui/home.html")
	t.Execute(wr, struct {
		Org     string
		IDToken string
	}{ASGARDEO_ORG_NAME, user.IDToken})
}

func auth(wr http.ResponseWriter, req *http.Request) {

	gothic.BeginAuthHandler(wr, req)
}

func addUserToSession(wr http.ResponseWriter, req *http.Request, user goth.User) {

	session, err := store.Get(req, sessionName)
	if err != nil {
		log.Print("Error ", err)
	}

	// Remove the raw data to reduce the size
	user.RawData = map[string]interface{}{}

	session.Values["user"] = user
	err = session.Save(req, wr)
	if err != nil {
		log.Print("Problem Saving session data", err)
	}
}

func removeUserFromSession(wr http.ResponseWriter, req *http.Request) {

	session, _ := store.Get(req, sessionName)
	session.Values["user"] = goth.User{}
	session.Save(req, wr)
}

func main() {

	store.MaxAge(3600)
	store.Options.Path = "/"
	store.Options.HttpOnly = true
	gothic.Store = store
	gob.Register(goth.User{})

	// Export these to enviroment varibles with your valus.
	OPENID_CONNECT_KEY := os.Getenv("OPENID_CONNECT_KEY")
	OPENID_CONNECT_SECRET := os.Getenv("OPENID_CONNECT_SECRET")
	ASGARDEO_ORG_NAME = os.Getenv("ASGARDEO_ORG_NAME")

	CALLBACK_URL := "http://localhost:8000/auth/openid-connect/callback"
	OPENID_CONNECT_DISCOVERY_URL := fmt.Sprintf("https://api.asgardeo.io/t/%s/oauth2/token/.well-known/openid-configuration", ASGARDEO_ORG_NAME)
	openidConnect, _ := openidConnect.New(OPENID_CONNECT_KEY, OPENID_CONNECT_SECRET, CALLBACK_URL, OPENID_CONNECT_DISCOVERY_URL)

	if openidConnect != nil {
		goth.UseProviders(openidConnect)
	}

	router := pat.New()
	router.Get("/auth/{provider}/callback", callback)
	router.Get("/auth/{provider}", auth)
	router.Get("/logout/{provider}", logout)
	router.Get("/home", home)
	router.Get("/", indexHandler)
	http.Handle("/", router)

	log.Print("Listening on port 8000...")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
