package main

import (
	"bufio"
	"crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
)

var (
	sessions   = make(map[string]string)
	sessionsMu sync.Mutex
	carts      = make(map[string][]part)
	cartsMu    sync.Mutex

	to = []string{"glennwarden@yahoo.com", "john@jjdev.com"}

	// to = []string{"john@jjdev.com"}
	senderPass = os.Getenv("SMTPPASS")
)

const (
	smtpServer = "smtp.gmail.com"
	smtpPort   = "587"
	senderMail = "john@jjdev.com"
)

type part interface {
	choices(string) []string
}

type Nipple struct {
	Size     string
	Quantity int
}

func (Nipple) choices(string) []string {
	return nippleSizes
}

var nippleSizes = func() []string {
	sizes := []string{"close"}
	for inches := 2; inches <= 12; inches++ {
		sizes = append(sizes, fmt.Sprintf("1″ × %d″", inches))
		if inches < 12 {
			sizes = append(sizes, fmt.Sprintf("1″ × %d½″", inches))
		}
	}
	return sizes
}()

type Pipe struct {
	ThicknessType string
	CoatingType   string
	EndType       string
	Size          string
	Feet          int
}

func (Pipe) choices(field string) []string {
	return pipeChoices[field]
}

type MechanicalTee struct {
	Size     string
	Quantity int
}

func (MechanicalTee) choices(string) []string {
	return mechanicalTeeSizes
}

type ScrewedTee struct {
	RunSize      string
	BullheadSize string
	Quantity     int
}

func (ScrewedTee) choices(field string) []string {
	return screwedTeeChoices[field]
}

var screwedTeeChoices = func() map[string][]string {
	outlets := []string{"2½″", "2″", "1½″", "1¼″", "1″", "½″"}
	var sizes []string
	for _, first := range outlets {
		for _, second := range outlets {
			sizes = append(sizes, first+" × "+second)
		}
	}
	return map[string][]string{
		"RunSize":      sizes,
		"BullheadSize": outlets,
	}
}()

type PipeStand struct {
	Size     string
	Quantity int
}

func (PipeStand) choices(string) []string {
	return pipeStandSizes
}

var pipeStandSizes = []string{
	"6″",
	"4″",
	"3″",
	"2½″",
	"2″",
	"1½″",
	"1¼″",
	"1″",
}

type GroovedCoupling struct {
	Type     string
	Size     string
	Quantity int
}

func (GroovedCoupling) choices(field string) []string {
	return groovedCouplingChoices[field]
}

var groovedCouplingChoices = map[string][]string{
	"Type": {"rigid", "flexible"},
	"Size": {"8″", "6″", "4″", "3″", "2½″", "2″", "1½″", "1¼″", "1″"},
}

type GroovedReducingCoupling struct {
	Size     string
	Quantity int
}

func (GroovedReducingCoupling) choices(string) []string {
	return groovedReducingCouplingSizes
}

var groovedReducingCouplingSizes = []string{
	"8″ × 6″",
	"6″ × 4″",
	"4″ × 3″",
	"4″ × 2½″",
	"4″ × 2″",
	"3″ × 2½″",
	"3″ × 2″",
	"2½″ × 2″",
	"2″ × 1¼″",
	"2″ × 1″",
	"2″ × ½″",
}

type GroovedFlangeAdapter struct {
	Size     string
	Quantity int
}

func (GroovedFlangeAdapter) choices(string) []string {
	return groovedFlangeAdapterSizes
}

var groovedFlangeAdapterSizes = []string{
	"12″", "10″", "8″", "6″", "4″", "3″", "2½″", "2″",
}

func formatPart(p part) string {
	value := reflect.ValueOf(p)
	partType := value.Type()
	var fields []string
	for index := range value.NumField() {
		field := partType.Field(index)
		fields = append(fields, fmt.Sprintf("%s: %v", field.Name, value.Field(index).Interface()))
	}
	return partType.Name() + ": " + strings.Join(fields, ", ")
}

func mapForm[T part](r *http.Request, p *T) map[string]error {
	errs := make(map[string]error)
	for field, value := range reflect.ValueOf(p).Elem().Fields() {
		input := r.Form.Get(field.Name)
		if input == "" {
			errs[field.Name] = errors.New("required")
			continue
		}
		switch value.Kind() {
		case reflect.String:
			if !slices.Contains((*p).choices(field.Name), input) {
				errs[field.Name] = fmt.Errorf("invalid value %q provided", input)
				continue
			}
			value.SetString(input)
		case reflect.Int:
			number, err := parsePositiveInt(input)
			if err != nil {
				errs[field.Name] = err
				continue
			}
			value.SetInt(int64(number))
		}
	}
	return errs
}

func parsePositiveInt(text string) (int, error) {
	number, err := strconv.Atoi(text)
	if err != nil {
		return 0, errors.New("value must be a whole number within the supported range")
	}
	if number < 1 {
		return 0, errors.New("must be greater than 0")
	}
	return number, nil
}

var pipeChoices = map[string][]string{
	"ThicknessType": {
		"sch 10",
		"sch 40",
	},
	"CoatingType": {
		"regular",
		"galvanized",
	},
	"EndType": {
		"plain end",
		"threaded",
		"grooved",
	},
	"Size": {
		"8″",
		"6″",
		"4″",
		"3″",
		"2½″",
		"2″",
		"1½″",
		"1¼″",
		"1″",
	},
}

var mechanicalTeeSizes = []string{
	"8″ × 4″",
	"8″ × 3″",
	"8″ × 2½″",
	"8″ × 2″",
	"8″ × 1½″",
	"8″ × 1¼″",
	"8″ × 1″",
	"6″ × 4″",
	"6″ × 3″",
	"6″ × 2½″",
	"6″ × 2″",
	"6″ × 1½″",
	"6″ × 1¼″",
	"6″ × 1″",
	"6″ × ¾″",
	"4″ × 2½″",
	"4″ × 2″",
	"4″ × 1½″",
	"4″ × 1¼″",
	"4″ × 1″",
	"4″ × ¾″",
	"3″ × 2″",
	"3″ × 1½″",
	"3″ × 1¼″",
	"3″ × 1″",
	"3″ × ¾″",
	"2½″ × 1½″",
	"2½″ × 1¼″",
	"2½″ × 1″",
	"2½″ × ¾″",
	"2″ × 1¼″",
	"2″ × 1″",
	"2″ × ¾″",
	"2″ × ½″",
}

type formField struct {
	Name, Value string
	Choices     []string
	Error       error
}

const pageStart = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>pot part ordering tool</title>
<style>
html { font-size: 150%; }
body { margin: 0; padding: 0.2rem; font-family: sans-serif; line-height: 1.4; overflow-wrap: anywhere; }
*, *::before, *::after { box-sizing: border-box; }
h1 { font-size: 1.25rem; margin: 0 0 0.4rem; }
form { margin-bottom: 0.4rem; }
form > div { margin-bottom: 0.3rem; }
label { display: block; }
input, select, button { font: inherit; max-width: 100%; min-width: 44px; min-height: 44px; padding: 0.1em; }
input, select { display: block; width: 100%; }
button { white-space: normal; overflow-wrap: anywhere; }
a { display: inline-block; min-width: 44px; min-height: 44px; padding-block: 0.1em; }
</style>
</head>
<body>
`

const pageEnd = `</body></html>`

var partTemplate = template.Must(template.New("part").Parse(pageStart + `
<h1>{{.PartName}}</h1>
<form action="{{.Target}}" method="post">
{{range .Fields}}
<div>
  <label for="{{.Name}}">{{.Name}}</label>
  {{if .Choices}}
  <select id="{{.Name}}" name="{{.Name}}" required>
    {{$value := .Value}}{{range .Choices}}
    <option value="{{.}}" {{if eq $value .}}selected{{end}}>{{.}}</option>
    {{end}}
  </select>
  {{else}}
  <input type="number" id="{{.Name}}" name="{{.Name}}" value="{{.Value}}" min="1" required>
  {{end}}
  {{with .Error}}<div role="alert">{{.}}</div>{{end}}
</div>
{{end}}
<button type="submit">save part</button>
</form>
{{if .Initial}}
<script>
document.querySelectorAll('select').forEach(select => select.selectedIndex = -1);
</script>
{{end}}
` + pageEnd))

func partForm[T part](w http.ResponseWriter, r *http.Request) {
	var p T
	var errs map[string]error
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		errs = mapForm(r, &p)
		if len(errs) == 0 {
			log.Printf("post %s", formatPart(p))
			id := sessionID(w, r)
			cartsMu.Lock()
			carts[id] = append(carts[id], p)
			cartsMu.Unlock()
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	var fields []formField
	value := reflect.ValueOf(p)
	for index := range value.NumField() {
		field := value.Type().Field(index)
		input := formField{
			Name:  field.Name,
			Value: fmt.Sprint(value.Field(index).Interface()),
			Error: errs[field.Name],
		}
		if r.Method == http.MethodPost {
			input.Value = r.Form.Get(field.Name)
		}
		if value.Field(index).Kind() == reflect.String {
			input.Choices = p.choices(field.Name)
		}
		fields = append(fields, input)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(errs) > 0 {
		w.WriteHeader(http.StatusBadRequest)
	}

	if err := partTemplate.Execute(w, struct {
		PartName string
		Target   string
		Fields   []formField
		Initial  bool
	}{
		PartName: value.Type().Name(),
		Target:   r.URL.Path, Fields: fields,
		Initial: r.Method == http.MethodGet,
	}); err != nil {
		log.Println(err)
	}
}

func readUsers(r io.Reader) (map[string]string, error) {
	users := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		username, password, found := strings.Cut(scanner.Text(), "|")
		if !found {
			return nil, fmt.Errorf("passwd:%d: expected username|password", line)
		}
		users[username] = password
	}
	return users, scanner.Err()
}
func requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var username string
		loggedIn := false
		if cookie, err := r.Cookie("sid"); err == nil {
			sessionsMu.Lock()
			username, loggedIn = sessions[cookie.Value]
			sessionsMu.Unlock()
		}

		log.Printf("%s %s %s user=%q", r.Method, r.URL.Path, r.RemoteAddr, username)
		if !loggedIn {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
func login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	log.Println(r.URL.Path, r.Method, r.RemoteAddr)
	io.WriteString(w, pageStart+`<form method="post" action="/login">
	<input name="username" placeholder="username" required autofocus><br>
        <input name="password" type="password" placeholder="password" required><br>
        <button type="submit">login</button>
    </form>`+pageEnd)
}

func authenticate(users map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path, r.Method, r.RemoteAddr)
		username := r.FormValue("username")
		if password, ok := users[username]; !ok || password != r.FormValue("password") {
			http.Error(w, "invalid login", http.StatusUnauthorized)
			return
		}

		id := sessionID(w, r)

		sessionsMu.Lock()
		sessions[id] = username
		sessionsMu.Unlock()

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	id := sessionID(w, r)

	cartsMu.Lock()
	items := slices.Clone(carts[id])
	cartsMu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	io.WriteString(w, pageStart+`
				<h1>
				<a href="./nipple">nipple</a><br>
				<a href="./pipe">pipe</a><br>
				<a href="./mechanical-tee">mechanical tee</a><br>
				<a href="./screwed-tee">screwed tee</a><br>
				<a href="./pipe-stand">pipe stand</a><br>
				<a href="./grooved-coupling">grooved coupling</a><br>
				<a href="./grooved-reducing-coupling">grooved reducing coupling</a><br>
				<a href="./grooved-flange-adapter">grooved flange adapter</a><br>
				</h1>
		`)
	for index, item := range items {
		io.WriteString(w, `<form action="/remove" method="post"><p>`)
		io.WriteString(w, template.HTMLEscapeString(formatPart(item)))
		fmt.Fprintf(w, ` <button type="submit" name="index" value="%d">delete</button></p></form>`, index)
	}

	if len(items) > 0 {
		io.WriteString(w, `
			<form action="/submit" method="post">
			    <button type="submit">submit order</button>
			</form>
		`)
	}
	io.WriteString(w, pageEnd)
}

func removePart(w http.ResponseWriter, r *http.Request) {
	id := sessionID(w, r)
	index, err := strconv.Atoi(r.FormValue("index"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cartsMu.Lock()
	cart := carts[id]
	if index < 0 || index >= len(cart) {
		cartsMu.Unlock()
		http.NotFound(w, r)
		return
	}
	carts[id] = slices.Delete(cart, index, index+1)
	cartsMu.Unlock()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func sendOrder(o order) error {
	return smtp.SendMail(
		smtpServer+":"+smtpPort,
		smtp.PlainAuth("", senderMail, senderPass, smtpServer),
		senderMail,
		to,
		[]byte(fmt.Sprintf("subject: order alert\n\n%s\nrequester: %s", o, o.username)),
	)
}

type order struct {
	id, username string
	items        []part
}

func (o order) String() string {
	var text strings.Builder
	for _, item := range o.items {
		text.WriteString(formatPart(item))
		text.WriteByte('\n')
	}
	return text.String()
}

func submit(recordOrder func(string, ...any) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := sessionID(w, r)
		cartsMu.Lock()
		items := carts[id]
		delete(carts, id)
		cartsMu.Unlock()
		if len(items) == 0 {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		sessionsMu.Lock()
		username := sessions[id]
		sessionsMu.Unlock()
		o := order{id: rand.Text(), username: username, items: items}

		if err := recordOrder(
			"%s: submitted by %q: %q",
			o.id,
			o.username,
			strings.TrimSuffix(o.String(), "\n"),
		); err != nil {
			log.Printf("ERROR %s: %q", o.id, err.Error())
		}

		err := sendOrder(o)
		status := "sent"
		if err != nil {
			log.Printf("ERROR %s: %q", o.id, err.Error())
			status = fmt.Sprintf("failed: %v", err)
		}
		if logErr := recordOrder("%s: email %q", o.id, status); logErr != nil {
			log.Printf("ERROR %s: email %q: %q", o.id, status, logErr.Error())
		}
		if err != nil {
			cartsMu.Lock()
			carts[id] = append(o.items, carts[id]...)
			cartsMu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, pageStart+`
			    <div>
				<h1>sent</h1>
				<a href="/">&larr; back</a>
			    </div>
			`+pageEnd)
	}
}

func sessionID(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie("sid"); err == nil {
		return c.Value
	}

	id := rand.Text()
	http.SetCookie(w, &http.Cookie{
		Name: "sid", Value: id, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	return id
}

func main() {
	ordersLog := log.New(os.Stderr, "", log.LstdFlags)
	ordersFile, err := os.OpenFile("orders.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		log.Printf("ERROR %q", err.Error())
		ordersLog.SetPrefix(fmt.Sprintf("ERROR %q: ", err.Error()))
	} else {
		defer ordersFile.Close()
		ordersLog.SetOutput(ordersFile)
	}

	recordOrder := func(format string, args ...any) error {
		if err := ordersLog.Output(2, fmt.Sprintf(format, args...)); err != nil {
			return err
		}
		if ordersFile != nil {
			return ordersFile.Sync()
		}
		return nil
	}

	f, err := os.Open("passwd")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	users, err := readUsers(f)
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("GET /login", login)

	http.HandleFunc("POST /login", authenticate(users))
	http.HandleFunc("GET /nipple", requireLogin(partForm[Nipple]))
	http.HandleFunc("POST /nipple", requireLogin(partForm[Nipple]))
	http.HandleFunc("GET /pipe", requireLogin(partForm[Pipe]))
	http.HandleFunc("POST /pipe", requireLogin(partForm[Pipe]))
	http.HandleFunc("GET /mechanical-tee", requireLogin(partForm[MechanicalTee]))
	http.HandleFunc("POST /mechanical-tee", requireLogin(partForm[MechanicalTee]))
	http.HandleFunc("GET /screwed-tee", requireLogin(partForm[ScrewedTee]))
	http.HandleFunc("POST /screwed-tee", requireLogin(partForm[ScrewedTee]))
	http.HandleFunc("GET /pipe-stand", requireLogin(partForm[PipeStand]))
	http.HandleFunc("POST /pipe-stand", requireLogin(partForm[PipeStand]))
	http.HandleFunc("GET /grooved-coupling", requireLogin(partForm[GroovedCoupling]))
	http.HandleFunc("POST /grooved-coupling", requireLogin(partForm[GroovedCoupling]))
	http.HandleFunc("GET /grooved-reducing-coupling", requireLogin(partForm[GroovedReducingCoupling]))
	http.HandleFunc("POST /grooved-reducing-coupling", requireLogin(partForm[GroovedReducingCoupling]))
	http.HandleFunc("GET /grooved-flange-adapter", requireLogin(partForm[GroovedFlangeAdapter]))
	http.HandleFunc("POST /grooved-flange-adapter", requireLogin(partForm[GroovedFlangeAdapter]))

	http.HandleFunc("GET /{$}", requireLogin(index))
	http.HandleFunc("POST /remove", requireLogin(removePart))
	http.HandleFunc("POST /submit", requireLogin(submit(recordOrder)))
	go func() {
		redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r,
				"https://jjdev.com"+r.URL.RequestURI(),
				http.StatusPermanentRedirect)
		})
		log.Fatal(http.ListenAndServe(":8080", redirect))
	}()

	log.Fatal(http.ListenAndServeTLS(
		":8443",
		"/etc/letsencrypt/live/jjdev.com/fullchain.pem",
		"/etc/letsencrypt/live/jjdev.com/privkey.pem",
		nil,
	))
}
