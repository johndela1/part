package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
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
	carts      = make(map[string][]any)
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

type Nipple struct {
	Size     string
	Quantity int
}

var nippleSizes = []string{
	"close",
	"1″ × 2″",
	"1″ × 2½″",
	"1″ × 3″",
	"1″ × 3½″",
	"1″ × 4″",
	"1″ × 4½″",
	"1″ × 5″",
	"1″ × 5½″",
	"1″ × 6″",
	"1″ × 6½″",
	"1″ × 7″",
	"1″ × 7½″",
	"1″ × 8″",
	"1″ × 8½″",
	"1″ × 9″",
	"1″ × 9½″",
	"1″ × 10″",
	"1″ × 10½″",
	"1″ × 11″",
	"1″ × 11½″",
	"1″ × 12″",
}

type Pipe struct {
	ThicknessType string
	CoatingType   string
	EndType       string
	Size          string
	Quantity      int
}

type MechanicalTee struct {
	Size     string
	Quantity int
}

func fieldLabel(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "Type", " type"))
}

func formatPart(part any) string {
	value := reflect.ValueOf(part)
	partType := value.Type()
	var fields []string
	for index := range value.NumField() {
		field := partType.Field(index)
		label := fieldLabel(field.Name)
		fields = append(fields, fmt.Sprintf("%s: %v", label, value.Field(index).Interface()))
	}
	return strings.ToLower(partType.Name() + ": " + strings.Join(fields, ", "))
}

func mapForm[T any](r *http.Request, part *T) map[string]error {
	errors := make(map[string]error)
	value := reflect.ValueOf(part).Elem()
	for index := range value.NumField() {
		field := value.Type().Field(index)
		text := r.Form.Get(field.Name)
		if err := validateRequired(text); err != nil {
			errors[field.Name] = err
			continue
		}
		switch value.Field(index).Kind() {
		case reflect.String:
			if err := validateChoice(text, choicesFor(*part, field.Name)); err != nil {
				errors[field.Name] = err
				continue
			}
			value.Field(index).SetString(text)
		case reflect.Int:
			quantity, err := parseQuantity(text)
			if err != nil {
				errors[field.Name] = err
				continue
			}
			value.Field(index).SetInt(int64(quantity))
		}
	}
	return errors
}

func validateRequired(value string) error {
	if value == "" {
		return errors.New("This field is required")
	}
	return nil
}

func parseQuantity(text string) (int, error) {
	quantity, err := strconv.Atoi(text)
	if err != nil {
		return 0, errors.New("Quantity must be a whole number within the supported range")
	}
	if quantity < 1 {
		return 0, errors.New("Value should be greater than or equal to 1")
	}
	return quantity, nil
}

func validateChoice(value string, choices []string) error {
	if !slices.Contains(choices, value) {
		return fmt.Errorf("Invalid value '%s' provided", value)
	}
	return nil
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

func choicesFor(part any, field string) []string {
	switch part.(type) {
	case Nipple:
		return nippleSizes
	case Pipe:
		return pipeChoices[field]
	case MechanicalTee:
		return mechanicalTeeSizes
	}
	return nil
}

type formField struct {
	Name, Value string
	Choices     []string
	Error       error
}

var partTemplate = template.Must(template.New("part").Funcs(template.FuncMap{
	"label": fieldLabel,
}).Parse(`
<form action="{{.Target}}" method="post">
{{range .Fields}}
<div>
  <label for="{{.Name}}">{{label .Name}} <span aria-hidden="true">*</span></label>
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
`))

func partForm[T any](w http.ResponseWriter, r *http.Request) {
	var part T
	var errors map[string]error
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
		errors = mapForm(r, &part)
		if len(errors) == 0 {
			log.Printf("post %s", formatPart(part))
			id := sessionID(w, r)
			cartsMu.Lock()
			carts[id] = append(carts[id], part)
			cartsMu.Unlock()
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	var fields []formField
	value := reflect.ValueOf(part)
	for index := range value.NumField() {
		field := value.Type().Field(index)
		input := formField{
			Name:  field.Name,
			Value: fmt.Sprint(value.Field(index).Interface()),
			Error: errors[field.Name],
		}
		if r.Method == http.MethodPost {
			input.Value = r.Form.Get(field.Name)
		}
		if value.Field(index).Kind() == reflect.String {
			input.Choices = choicesFor(part, field.Name)
		}
		fields = append(fields, input)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(errors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
	}

	if err := partTemplate.Execute(w, struct {
		Target  string
		Fields  []formField
		Initial bool
	}{
		Target: r.URL.Path, Fields: fields,
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
	io.WriteString(w, `<form method="post" action="/login">
	<input name="username" placeholder="username" required autofocus><br>
        <input name="password" type="password" placeholder="password" required><br>
        <button type="submit">login</button>
    </form>`)
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

	io.WriteString(w, `
			<!DOCTYPE html>
			<html>
			<body>
				<h1>
				<a href="./nipple">nipple</a><br>
				<a href="./pipe">pipe</a><br>
				<a href="./mechanical-tee">mechanical tees</a><br>
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
		</body>
		</html>

		`)
	}
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
		[]byte("subject: order alert\n\n"+o.parts+"\nrequester: "+o.username),
	)
}

type order struct {
	id, username, parts string
	items               []any
}

func takeCart(id string) (order, bool) {
	cartsMu.Lock()
	cart := carts[id]
	if len(cart) == 0 {
		cartsMu.Unlock()
		return order{}, false
	}

	var parts string
	for _, item := range cart {
		parts += formatPart(item) + "\n"
	}
	delete(carts, id)
	cartsMu.Unlock()

	sessionsMu.Lock()
	username := sessions[id]
	sessionsMu.Unlock()
	return order{
		id: rand.Text(), username: username, parts: parts, items: cart,
	}, true
}

func submit(recordOrder func(string, ...any) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := sessionID(w, r)
		o, ok := takeCart(id)
		if !ok {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		if err := recordOrder(
			"%s: submitted by %q: %q",
			o.id,
			o.username,
			strings.TrimSuffix(o.parts, "\n"),
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

		io.WriteString(w, `
			<!DOCTYPE html>
			<html>
			<head>
			    <title>pot part ordering tool</title>
			</head>
			<body>
			    <div>
				<h1>sent</h1>
				<a href="/">&larr; back</a>
			    </div>
			</body>
			</html>
			`)
	}
}

func sessionID(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie("sid"); err == nil {
		return c.Value
	}

	b := make([]byte, 32)
	rand.Read(b)

	id := hex.EncodeToString(b)
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
