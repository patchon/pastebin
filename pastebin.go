// Package pastebin is a simple modern and powerful pastebin service
package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	// Random string generation,
	"github.com/dchest/uniuri"

	// Database drivers,
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	// For url routing
	"github.com/gorilla/mux"
)

// Configuration struct,
type Configuration struct {
	DBHost         string    `json:"dbhost"`     // Name of your database host
	DBName         string    `json:"dbname"`     // Name of your database
	DBPassword     string    `json:"dbpassword"` // The password for the database user
	DBPlaceHolder  [6]string // ? / $[i] Depending on db driver.
	DBPort         string    `json:"dbport"`                // Port of the database
	DBTable        string    `json:"dbtable"`               // Name of the table in the database
	DBType         string    `json:"dbtype"`                // Type of database
	DBUser         string    `json:"dbuser"`                // The database user
	DisplayName    string    `json:"displayname"`           // Name of your pastebin
	Highlighter    string    `json:"highlighter"`           // The name of the highlighter.
	ListenAddress  string    `json:"listenaddress"`         // Address that pastebin will bind on
	ListenPort     string    `json:"listenport"`            // Port that pastebin will listen on
	ShortUrlLength int       `json:"shorturllength,string"` // Length of the generated short urls
}

// This struct is used for responses.
// A request to the pastebin will always this json struct.
type Response struct {
	DelKey string `json:"delkey"` // The id to use when delete a paste
	Expiry string `json:"expiry"` // The date when post expires
	Extra  string `json:"extra"`  // Extra output from the highlight-wrapper
	Id     string `json:"id"`     // The id of the paste
	Lang   string `json:"lang"`   // Specified language
	Paste  string `json:"paste"`  // The eactual paste data
	Sha1   string `json:"sha1"`   // The sha1 of the paste
	Size   int    `json:"size"`   // The length of the paste
	Status string `json:"status"` // A custom status message
	Style  string `json:"style"`  // Specified style
	Title  string `json:"title"`  // The title of the paste
	Url    string `json:"url"`    // The url of the paste
}

// This struct is used for indata when a request is being made to the pastebin.
type Request struct {
	DelKey string `json:"delkey"`        // The delkey that is used to delete paste
	Expiry int64  `json:"expiry,string"` // An expiry date
	Id     string `json:"id"`            // The id of the paste
	Lang   string `json:"lang"`          // The language of the paste
	Paste  string `json:"paste"`         // The actual pase
	Style  string `json:"style"`         // The style of the paste
	Title  string `json:"title"`         // The title of the paste
	WebReq bool   `json:"webreq"`        // If its a webrequest or not
}

// This struct is used for generating pages.
type Page struct {
	Body            template.HTML
	Expiry          string
	Lang            string
	LangsFirst      map[string]string
	LangsLast       map[string]string
	PasteId         string
	PasteTitle      string
	Style           string
	SupportedStyles map[string]string
	Title           string
	WrapperErr      string
}

// Template pages,
var templates = template.Must(template.ParseFiles("assets/index.html",
	"assets/syntax.html"))

// Global variables, *shrug*
var configuration Configuration
var dbHandle *sql.DB
var debug bool
var debugLogger *log.Logger
var listOfLangsFirst map[string]string
var listOfLangsLast map[string]string
var listOfStyles map[string]string
var gets int

//
// Functions below,
//

// loggy prints a message if the debug flag is turned.
func loggy(str string) {
	if debug {
		debugLogger.Println("   " + str)
	}
}

// checkErr simply checks if passed error is anything but nil.
// If an error exists it will be printed and the program terminates.
func checkErr(err error) {
	if err != nil {
		debugLogger.Println("   " + err.Error())
		os.Exit(1)
	}
}

// getSupportedStyless reads supported styles from the highlighter-wrapper
// (which in turn gets available styles from pygments). It then puts them into
// an array which is used by the html-template. The function doesn't return
// anything since the array is defined globally (shrug).
func getSupportedStyles() {

	listOfStyles = make(map[string]string)

	arg := "getstyles"
	out, err := exec.Command(configuration.Highlighter, arg).Output()
	if err != nil {
		log.Fatal(err)
	}

	// Loop lexers and add them to respectively map,
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}

		loggy(fmt.Sprintf("Populating supported styles map with %s", line))
		listOfStyles[line] = strings.Title(line)
	}
}

// getSupportedLangs reads supported lexers from the highlighter-wrapper (which
// in turn gets available lexers from pygments). It then puts them into two
// maps, depending on if it's a "prioritized" lexers. If it's prioritized or not
// is determined by if its listed in the assets/prio-lexers.  The description is
// the key and the actual lexer is the value. The maps are used by the
// html-template. The function doesn't return anything since the maps are
// defined globally (shrug).
func getSupportedLangs() {

	var prioLexers map[string]string

	// Initialize maps,
	prioLexers = make(map[string]string)
	listOfLangsFirst = make(map[string]string)
	listOfLangsLast = make(map[string]string)

	// Get prioritized lexers and put them in a separate map,
	file, err := os.Open("assets/prio-lexers")
	checkErr(err)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		prioLexers[scanner.Text()] = "1"
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	file.Close()

	arg := "getlexers"
	out, err := exec.Command(configuration.Highlighter, arg).Output()
	if err != nil {
		log.Fatal(err)
	}

	// Loop lexers and add them to respectively map,
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}

		s := strings.Split(line, ";")
		if len(s) != 2 {
			loggy(fmt.Sprintf("Could not split '%v' from %s (fields should be seperated by ;)",
				s, configuration.Highlighter))
			os.Exit(1)
		}
		s[0] = strings.Title(s[0])
		if prioLexers[s[0]] == "1" {
			loggy(fmt.Sprintf("Populating first languages map with %s - %s",
				s[0], s[1]))
			listOfLangsFirst[s[0]] = s[1]
		} else {
			loggy(fmt.Sprintf("Populating second languages map with %s - %s",
				s[0], s[1]))
			listOfLangsLast[s[0]] = s[1]
		}
	}
}

// printHelp prints a description of the program.
// Exit code will depend on how the function is called.
func printHelp(err int) {

	fmt.Printf("\n Description, \n")
	fmt.Printf("    - This is a small (< 600 line of go) pastebing with")
	fmt.Printf(" support for syntax highlightnig (trough python-pygments).\n")
	fmt.Printf("      No more no less.\n\n")

	fmt.Printf(" Usage, \n")
	fmt.Printf("    - %s [--help] [--debug]\n\n", os.Args[0])

	fmt.Printf(" Where, \n")
	fmt.Printf("    - help shows this incredibly useful help.\n")
	fmt.Printf("    - debug shows quite detailed information about whats")
	fmt.Printf(" going on.\n\n")

	os.Exit(err)
}

// checkArgs parses the command line in a very simple manner.
func checkArgs() {

	if len(os.Args[1:]) >= 1 {
		for _, arg := range os.Args[1:] {
			switch arg {
			case "-h", "--help":
				printHelp(0)
			case "-d", "--debug":
				debug = true
			default:
				printHelp(1)
			}
		}
	}
}

// getDbHandle opens a connection to database.
// Returns the dbhandle if the open was successful
func getDBHandle() *sql.DB {

	var dbinfo string
	for i := 0; i < 6; i++ {
		configuration.DBPlaceHolder[i] = "?"
	}

	switch configuration.DBType {

	case "sqlite3":
		dbinfo = configuration.DBName
		loggy("Specified databasetype : " + configuration.DBType)
		loggy(fmt.Sprintf("Trying to open %s (%s)",
			configuration.DBName, configuration.DBType))

	case "postgres":
		dbinfo = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			configuration.DBHost,
			configuration.DBPort,
			configuration.DBUser,
			configuration.DBPassword,
			configuration.DBName)
		for i := 0; i < 6; i++ {
			configuration.DBPlaceHolder[i] = "$" + strconv.Itoa(i+1)
		}

	case "mysql":
		dbinfo = configuration.DBUser + ":" + configuration.DBPassword + "@tcp(" + configuration.DBHost + ":" + configuration.DBPort + ")/" + configuration.DBName

	case "":
		debugLogger.Println("   Database error : dbtype not specified in configuration.")
		os.Exit(1)

	default:
		debugLogger.Println("   Database error : Specified dbtype (" +
			configuration.DBType + ") not supported.")
		os.Exit(1)
	}

	db, err := sql.Open(configuration.DBType, dbinfo)
	checkErr(err)

	// Just create a dummy query to really verify that the database is working as
	// expected,
	var dummy string
	err = db.QueryRow("select id from " + configuration.DBTable + " where id='dummyid'").Scan(&dummy)

	switch {
	case err == sql.ErrNoRows:
		loggy("Successfully connected and found table " + configuration.DBTable)
	case err != nil:
		debugLogger.Println("   Database error : " + err.Error())
		os.Exit(1)
	}

	return db
}

// generateName generates a short url with the length defined in main config
// The function calls itself recursively until an id that doesn't exist is found
// Returns the id
func generateName() string {

	// Use uniuri to generate random string
	id := uniuri.NewLen(configuration.ShortUrlLength)
	loggy(fmt.Sprintf("Generated id is '%s', checking if it's already taken in the database",
		id))

	// Query database if id exists and if it does call generateName again
	var id_taken string
	err := dbHandle.QueryRow("select id from "+configuration.DBTable+
		" where id="+configuration.DBPlaceHolder[0], id).
		Scan(&id_taken)

	switch {
	case err == sql.ErrNoRows:
		loggy(fmt.Sprintf("Id '%s' is not taken, will use it.", id))
	case err != nil:
		debugLogger.Println("   Database error : " + err.Error())
		os.Exit(1)
	default:
		loggy(fmt.Sprintf("Id '%s' is taken, generating new id.", id_taken))
		generateName()
	}

	return id
}

// shaPaste hashes the paste data into a sha1 hash which will be used to
// determine if the pasted data already exists in the database
// Returns the hash
func shaPaste(paste string) string {

	hasher := sha1.New()
	hasher.Write([]byte(paste))
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))

	loggy(fmt.Sprintf("Generated sha for paste is '%s'", sha))
	return sha
}

// savePaste handles the saving for each paste.
// Takes the arguments,
// title, title of the paste as string,
// paste, the actual paste data as a string,
// expiry, the epxpiry date in epoch time as an int64
// Returns the Response struct
func savePaste(title string, paste string, expiry int64, hostname string) Response {

	var id, hash, delkey, url string

	// Escape user input,
	paste = html.EscapeString(paste)
	title = html.EscapeString(title)

	// Hash paste data and query database to see if paste exists
	sha := shaPaste(paste)
	loggy("Checking if pasted data is already in the database.")

	err := dbHandle.QueryRow("select id, title, hash, data, delkey from "+
		configuration.DBTable+" where hash="+
		configuration.DBPlaceHolder[0], sha).Scan(&id,
		&title, &hash, &paste, &delkey)
	switch {
	case err == sql.ErrNoRows:
		loggy("Pasted data is not in the database, will insert it.")
	case err != nil:
		debugLogger.Println("   Database error : " + err.Error())
		os.Exit(1)
	default:
		loggy(fmt.Sprintf("Pasted data already exists at id '%s' with title '%s'.",
			id, html.UnescapeString(title)))

		return Response{
			Status: "Paste data already exists ...",
			Id:     id,
			Title:  title,
			Url:    url,
			Sha1:   hash,
			Size:   len(paste)}
	}

	// Generate id,
	id = generateName()
	url = hostname + "/p/" + id

	// Set expiry if it's specified,
	if expiry != 0 {
		expiry += time.Now().Unix()
	}

	// Set the generated id as title if not given,
	if title == "" {
		title = id
	}

	delKey := uniuri.NewLen(40)

	// This is needed since mysql/postgres uses different placeholders,
	var dbQuery string
	for i := 0; i < 6; i++ {
		dbQuery += configuration.DBPlaceHolder[i] + ","
	}
	dbQuery = dbQuery[:len(dbQuery)-1]

	stmt, err := dbHandle.Prepare("INSERT INTO " + configuration.DBTable + " (id,title,hash,data,delkey,expiry)values(" + dbQuery + ")")
	checkErr(err)

	_, err = stmt.Exec(id, title, sha, paste, delKey, expiry)
	checkErr(err)

	loggy(fmt.Sprintf("Sucessfully inserted data at id '%s', title '%s', expiry '%v' and data \n \n* * * *\n\n%s\n\n* * * *\n",
		id,
		html.UnescapeString(title),
		expiry,
		html.UnescapeString(paste)))
	stmt.Close()
	checkErr(err)

	return Response{
		Status: "Successfully saved paste.",
		Id:     id,
		Title:  title,
		Sha1:   hash,
		Url:    url,
		Size:   len(paste),
		DelKey: delKey}
}

// DelHandler handles the deletion of pastes.
// If pasteId and DelKey consist the paste will be removed.
func DelHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var inData Request

	inData.Id = vars["pasteId"]

	// Escape user input,
	inData.DelKey = html.EscapeString(inData.DelKey)
	inData.Id = html.EscapeString(inData.Id)

	fmt.Printf("Trying to delete paste with id '%s' and delkey '%s'\n",
		inData.Id, inData.DelKey)
	stmt, err := dbHandle.Prepare("delete from pastebin where delkey=" +
		configuration.DBPlaceHolder[0] + " and id=" +
		configuration.DBPlaceHolder[1])
	checkErr(err)
	res, err := stmt.Exec(inData.DelKey, inData.Id)
	checkErr(err)

	_, err = res.RowsAffected()

	if err != sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		b := Response{Status: "Deleted paste " + inData.Id}
		err := json.NewEncoder(w).Encode(b)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// SaveHandler will handle the actual save of each paste.
// Returns with a Response struct.
func SaveHandler(w http.ResponseWriter, r *http.Request) {
	loggy(fmt.Sprintf("url=%s host=%s method=%s", r.URL.String(), r.Host, r.Method))

	var inData Request

	loggy(fmt.Sprintf("Recieving request to save new paste, trying to parse indata."))
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&inData)

	// Return error if we can't decode the json-data,
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	d, _ := json.MarshalIndent(inData, "DEBUG : ", "  ")
	loggy(fmt.Sprintf("Successfully parsed json indata into struct \nDEBUG : %s", d))

	// Return error if we don't have any data at all
	if inData.Paste == "" {
		loggy("Empty paste received, returning 500.")
		http.Error(w, "Empty paste.", 500)
		return
	}

	// Return error if title is to long
	// TODO add check of paste size.
	if len(inData.Title) > 50 {
		loggy(fmt.Sprintf("Paste title to long (%v).", len(inData.Title)))
		http.Error(w, "Title to long.", 500)
		return
	}

	// Hmm, not sure why this why this is not in the request,
	scheme := "http://"
	if r.TLS != nil {
		scheme = "https://"
	}

	p := savePaste(inData.Title, inData.Paste, inData.Expiry, scheme+r.Host)

	d, _ = json.MarshalIndent(p, "DEBUG : ", "  ")
	loggy(fmt.Sprintf("Returning json data to requester \nDEBUG : %s", d))

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(p)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// high calls the highlighter-wrapper and runs the paste through it.
// Takes the arguments,
// paste, the actual paste data as a string,
// lang, the pygments lexer to use as a string,
// style, the pygments style to use as a string
// Returns two strings, first is the output from the pygments html-formatter,
// the second is a custom message
func high(paste string, lang string, style string) (string, string, string, string) {

	// Defaults
	var default_lang, default_style string
	default_lang = "autodetect"
	default_style = "manni"

	// Lets loop through the supported languages to catch if the user is doing
	// something fishy. We do this to be extra safe since we are making an
	// an external call with user input.
	var supported_lang, supported_style bool
	supported_lang = false
	supported_style = false

	if lang == "" {
		lang = default_lang
		supported_lang = true
		loggy(fmt.Sprintf("Language is not set, will set to '%s'", default_lang))
	} else {
		// Check for supported languages
		for _, v1 := range listOfLangsFirst {
			if lang == v1 {
				supported_lang = true
			}
		}

		for _, v2 := range listOfLangsLast {
			if lang == v2 {
				supported_lang = true
			}
		}

		if lang == default_lang {
			supported_lang = true
		}
	}

	if supported_lang {
		loggy(fmt.Sprintf("Language set to '%s'", lang))
	} else {
		loggy(fmt.Sprintf("Requested language ('%s') is not supported, using '%s' as default",
			lang, default_lang))
	}

	// Same with the styles,
	if style == "" {
		style = default_style
		supported_style = true
		loggy(fmt.Sprintf("Style is not set, will set to '%s'", default_style))
	} else {
		for _, s := range listOfStyles {
			if style == strings.ToLower(s) {
				supported_style = true
			}
		}
	}

	if supported_style {
		loggy(fmt.Sprintf("Style set to '%s'", style))
	} else {
		loggy(fmt.Sprintf("Requested style ('%s') is not supported, using '%s' as default",
			style, default_style))
	}

	if _, err := os.Stat(configuration.Highlighter); os.IsNotExist(err) {
		log.Fatal(err)
	}

	loggy(fmt.Sprintf("Executing command : %s %s %s", configuration.Highlighter,
		lang, style))
	cmd := exec.Command(configuration.Highlighter, lang, style)
	cmd.Stdin = strings.NewReader(paste)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		loggy(fmt.Sprintf("The highlightning feature failed, returning text. Error : %s", stderr.String()))
		return paste, "Internal Error, returning plain text.", lang, style
	}

	loggy(fmt.Sprintf("The wrapper returned the requested language (%s)", lang))
	return stdout.String(), stderr.String(), lang, style
}

// checkPasteExpiry checks if a paste is overdue.
// It takes the pasteId as sting and the expiry date as an int64 as arguments.
// If the paste is overdue it gets deleted and false is returned.
func checkPasteExpiry(pasteId string, expiry int64) bool {

	loggy("Checking if paste is overdue.")
	if expiry == 0 {
		loggy("Paste doesn't have a duedate.")
	} else {
		// Current time,
		now := time.Now().Unix()

		// Human friendly strings for logging,
		nowStr := time.Unix(now, 0).Format("2006-01-02 15:04:05")
		expiryStr := time.Unix(expiry, 0).Format("2006-01-02 15:04:05")
		loggy(fmt.Sprintf("Checking if paste is overdue (is %s later than %s).",
			nowStr, expiryStr))

		// If expiry is greater than current time, delete paste,
		if now >= expiry {
			loggy("User requested a paste that is overdue, deleting it.")
			delPaste(pasteId)
			return false
		}
	}

	return true
}

// delPaste deletes the actual paste.
// It takes the pasteId as sting as argument.
func delPaste(pasteId string) {

	// Prepare statement,
	stmt, err := dbHandle.Prepare("delete from pastebin where id=" +
		configuration.DBPlaceHolder[0])
	checkErr(err)

	// Execute it,
	_, err = stmt.Exec(pasteId)
	checkErr(err)

	stmt.Close()
	loggy("Successfully deleted paste.")
}

// getPaste gets the paste from the database.
// Takes the pasteid as a string argument.
// Returns the Response struct.
func getPaste(pasteId string) Response {

	var title, paste string
	var expiry int64

	err := dbHandle.QueryRow("select title, data, expiry from "+
		configuration.DBTable+" where id="+configuration.DBPlaceHolder[0],
		pasteId).Scan(&title, &paste, &expiry)

	switch {
	case err == sql.ErrNoRows:
		loggy("Requested paste doesn't exist.")
		return Response{Status: "Requested paste doesn't exist."}
	case err != nil:
		debugLogger.Println("   Database error : " + err.Error())
		os.Exit(1)
	}

	// Check if paste is overdue,
	if !checkPasteExpiry(pasteId, expiry) {
		return Response{Status: "Requested paste doesn't exist."}
	}

	// Unescape the saved data,
	paste = html.UnescapeString(paste)
	title = html.UnescapeString(title)

	expiryS := "Never"
	if expiry != 0 {
		expiryS = time.Unix(expiry, 0).Format("2006-01-02 15:04:05")
	}

	r := Response{
		Status: "Success",
		Id:     pasteId,
		Title:  title,
		Paste:  paste,
		Size:   len(paste),
		Expiry: expiryS}

	d, _ := json.MarshalIndent(r, "DEBUG : ", "  ")
	loggy(fmt.Sprintf("Returning data from getPaste \nDEBUG : %s", d))

	return r
}

// APIHandler handles all
func APIHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pasteId := vars["pasteId"]

	var inData Request
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&inData)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	loggy(fmt.Sprintf("Getting paste with id '%s' and lang '%s' and style '%s'.",
		pasteId, inData.Lang, inData.Style))

	// Get the actual paste data,
	p := getPaste(pasteId)
	if p.Status == "Requested paste doesn't exist." {
		notfoundHandler(w, pasteId)
		return
	}

	if inData.WebReq {
		// If no style is given, use default style,
		if inData.Style == "" {
			inData.Style = "manni"
			p.Url += "/" + inData.Style
		}

		// If no lang is given, use autodetect
		if inData.Lang == "" {
			inData.Lang = "autodetect"
			p.Url += "/" + inData.Lang
		}

		// Run it through the highgligther.,
		p.Paste, p.Extra, p.Lang, p.Style = high(p.Paste, inData.Lang, inData.Style)
	}

	d, _ := json.MarshalIndent(p, "DEBUG : ", "  ")
	loggy(fmt.Sprintf("Returning json data to requester \nDEBUG : %s", d))

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(p)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func notfoundHandler(w http.ResponseWriter, pasteId string) {
	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "application/json")
	resp := make(map[string]string)
	resp["404"] = "Paste with id " + pasteId + " not found"
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		loggy(fmt.Sprintf("Could not create 404 json (%s)", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(jsonResp)
}

// pasteHandler generates the html paste pages
func pasteHandler(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	pasteId := vars["pasteId"]
	lang := vars["lang"]
	style := vars["style"]

	loggy(fmt.Sprintf("Getting paste with id '%s' and lang '%s' and style '%s'.", pasteId, lang, style))

	// Get the actual paste data,
	p := getPaste(pasteId)
	if p.Status == "Requested paste doesn't exist." {
		notfoundHandler(w, pasteId)
		return
	}

	// Run it through the highgligther.,
	p.Paste, p.Extra, p.Lang, p.Style = high(p.Paste, lang, style)

	// Construct page struct
	page := &Page{
		Body:            template.HTML(p.Paste),
		Expiry:          p.Expiry,
		Lang:            p.Lang,
		LangsFirst:      listOfLangsFirst,
		LangsLast:       listOfLangsLast,
		PasteId:         pasteId,
		Style:           p.Style,
		SupportedStyles: listOfStyles,
		Title:           p.Title,
		WrapperErr:      p.Extra,
	}

	err := templates.ExecuteTemplate(w, "syntax.html", page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// CloneHandler handles generating the clone pages
func CloneHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pasteId := vars["pasteId"]

	p := getPaste(pasteId)
	if p.Status == "Requested paste doesn't exist." {
		notfoundHandler(w, pasteId)
		return
	}

	loggy(p.Paste)

	// Clone page struct
	page := &Page{
		Body:       template.HTML(p.Paste),
		PasteTitle: "Copy of " + p.Title,
		Title:      "Copy of " + p.Title,
	}

	err := templates.ExecuteTemplate(w, "index.html", page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// DownloadHandler forces downloads of selected pastes
func DownloadHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pasteId := vars["pasteId"]

	p := getPaste(pasteId)
	if p.Status == "Requested paste doesn't exist." {
		notfoundHandler(w, pasteId)
		return
	}

	// Set header to an attachment so browser will automatically download it
	w.Header().Set("Content-Disposition", "attachment; filename="+p.Title)
	w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
	io.WriteString(w, p.Paste)
}

// RawHandler displays the pastes in text/plain format
func RawHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pasteId := vars["pasteId"]

	p := getPaste(pasteId)
	if p.Status == "Requested paste doesn't exist." {
		notfoundHandler(w, pasteId)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8; imeanit=yes")

	// Simply write string to browser
	io.WriteString(w, p.Paste)
}

// RootHandler handles generating the root page
func RootHandler(w http.ResponseWriter, r *http.Request) {

	p := &Page{
		LangsFirst: listOfLangsFirst,
		LangsLast:  listOfLangsLast,
		Title:      configuration.DisplayName,
	}

	gets++
	
	err := templates.ExecuteTemplate(w, "index.html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// MetricsHandler handles generating metrics
func MetricsHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8; imeanit=yes")

	str := "http_requests_total{method=\"any\"} "
	str += strconv.Itoa(gets)

	io.WriteString(w, str)
}

func serveCss(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "assets/pastebin.css")
}

func main() {

	// Set up new logger,
	debugLogger = log.New(os.Stderr, "DEBUG : ", log.Ldate|log.Ltime)

	// Check args,
	checkArgs()

	// Load config,
	file, err := os.Open("config.json")
	if err != nil {
		loggy(fmt.Sprintf("Error opening config.json (%s)", err))
		os.Exit(1)
	}
	loggy(fmt.Sprintf("Successfully opened %s", "config.json"))

	// Try to parse json,
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&configuration)
	if err != nil {
		loggy(fmt.Sprintf("Error parsing json data from %s : %s", "config.json", err))
		os.Exit(1)
	}

	d, _ := json.MarshalIndent(configuration, "DEBUG : ", "  ")
	loggy(fmt.Sprintf("Successfully parsed json data into struct \nDEBUG : %s", d))

	// Get languages and styles,
	getSupportedLangs()
	getSupportedStyles()

	// Get the database handle
	dbHandle = getDBHandle()

	// Router object,
	router := mux.NewRouter()

	// Routes,
	router.HandleFunc("/", RootHandler)
	router.HandleFunc("/p/{pasteId}", pasteHandler).Methods("GET")
	router.HandleFunc("/p/{pasteId}/{lang}", pasteHandler).Methods("GET")
	router.HandleFunc("/p/{pasteId}/{lang}/{style}", pasteHandler).Methods("GET")

	// Api
	router.HandleFunc("/api", SaveHandler).Methods("POST")
	router.HandleFunc("/api/{pasteId}", APIHandler).Methods("POST")
	router.HandleFunc("/api/{pasteId}", APIHandler).Methods("GET")
	router.HandleFunc("/api/{pasteId}", DelHandler).Methods("DELETE")

	router.HandleFunc("/raw/{pasteId}", RawHandler).Methods("GET")
	router.HandleFunc("/clone/{pasteId}", CloneHandler).Methods("GET")

	router.HandleFunc("/download/{pasteId}", DownloadHandler).Methods("GET")
	router.HandleFunc("/assets/pastebin.css", serveCss).Methods("GET")

	// Metrics
	router.HandleFunc("/metrics", MetricsHandler)
	
	// Set up server,
	srv := &http.Server{
		Handler:      router,
		Addr:         configuration.ListenAddress + ":" + configuration.ListenPort,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	err = srv.ListenAndServe()
	checkErr(err)
}
