package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
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

	"github.com/donseba/go-form/v2"
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
	Size     string `form:"dropdown" label:"size" values:"close:close;1″ × 2″:1″ × 2″;1″ × 2½″:1″ × 2½″;1″ × 3″:1″ × 3″;1″ × 3½″:1″ × 3½″;1″ × 4″:1″ × 4″;1″ × 4½″:1″ × 4½″;1″ × 5″:1″ × 5″;1″ × 5½″:1″ × 5½″;1″ × 6″:1″ × 6″;1″ × 6½″:1″ × 6½″;1″ × 7″:1″ × 7″;1″ × 7½″:1″ × 7½″;1″ × 8″:1″ × 8″;1″ × 8½″:1″ × 8½″;1″ × 9″:1″ × 9″;1″ × 9½″:1″ × 9½″;1″ × 10″:1″ × 10″;1″ × 10½″:1″ × 10½″;1″ × 11″:1″ × 11″;1″ × 11½″:1″ × 11½″;1″ × 12″:1″ × 12″;" required:"true"`
	Quantity int    `form:"input,number" label:"quantity" required:"true" min:"1"`
}

type Pipe struct {
	ThicknessType string `form:"dropdown" label:"thickness type" values:"SCH 10:sch 10;SCH 40:sch 40"`
	CoatingType   string `form:"dropdown" label:"coating type" values:"Regular:regular;Galvanized:galvanized"`
	EndType       string `form:"dropdown" label:"end type" values:"Plain End:plain end;Threaded:threaded;Grooved:grooved"`
	Size          string `form:"dropdown" label:"size" values:"8″:8″;6″:6″;4″:4″;3″:3″;2½″:2½″;2″:2″;1½″:1½″;1¼″:1¼″;1″:1″"`
	Quantity      int    `form:"input,number" label:"quantity" required:"true" min:"1"`
}

type MechanicalTee struct {
	Size     string `form:"dropdown" label:"size" values:"8″ × 4″:8″ × 4″;8″ × 3″:8″ × 3″;8″ × 2½″:8″ × 2½″;8″ × 2″:8″ × 2″;8″ × 1½″:8″ × 1½″;8″ × 1¼″:8″ × 1¼″;8″ × 1″:8″ × 1″;6″ × 4″:6″ × 4″;6″ × 3″:6″ × 3″;6″ × 2½″:6″ × 2½″;6″ × 2″:6″ × 2″;6″ × 1½″:6″ × 1½″;6″ × 1¼″:6″ × 1¼″;6″ × 1″:6″ × 1″;6″ × ¾″:6″ × ¾″;4″ × 2½″:4″ × 2½″;4″ × 2″:4″ × 2″;4″ × 1½″:4″ × 1½″;4″ × 1¼″:4″ × 1¼″;4″ × 1″:4″ × 1″;4″ × ¾″:4″ × ¾″;3″ × 2″:3″ × 2″;3″ × 1½″:3″ × 1½″;3″ × 1¼″:3″ × 1¼″;3″ × 1″:3″ × 1″;3″ × ¾″:3″ × ¾″;2½″ × 1½″:2½″ × 1½″;2½″ × 1¼″:2½″ × 1¼″;2½″ × 1″:2½″ × 1″;2½″ × ¾″:2½″ × ¾″;2″ × 1¼″:2″ × 1¼″;2″ × 1″:2″ × 1″;2″ × ¾″:2″ × ¾″;2″ × ½″:2″ × ½″" required:"true"`
	Quantity int    `form:"input,number" label:"quantity" required:"true" min:"1"`
}

func formatPart(part any) string {
	value := reflect.ValueOf(part)
	partType := value.Type()
	var fields []string
	for index := range value.NumField() {
		field := partType.Field(index)
		label := field.Tag.Get("label")
		fields = append(fields, fmt.Sprintf("%s: %v", label, value.Field(index).Interface()))
	}
	return strings.ToLower(partType.Name() + ": " + strings.Join(fields, ", "))
}

func mapForm[T any](r *http.Request, part *T) {
	value := reflect.ValueOf(part).Elem()
	partType := value.Type()
	for index := range value.NumField() {
		field := value.Field(index)
		text := r.FormValue(partType.Field(index).Name)
		switch field.Kind() {
		case reflect.String:
			field.SetString(text)
		case reflect.Int:
			if number, err := strconv.ParseInt(text, 10, 64); err == nil {
				field.SetInt(number)
			}
		}
	}
}

func partForm[T any](w http.ResponseWriter, r *http.Request) {
	var part T
	var errors form.FieldErrors
	renderer := form.NewForm()
	renderer.SetTheme("plain")
	log.Println(r.URL.Path, r.Method, r.RemoteAddr)
	if r.Method == http.MethodPost {
		if err := form.MapForm(r, &part); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("post %s", formatPart(part))
		errors = renderer.ValidateForm(&part)
		if len(errors) == 0 {
			id := sessionID(w, r)
			cartsMu.Lock()
			carts[id] = append(carts[id], part)
			cartsMu.Unlock()
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	model := form.WithRequestInfo(r, part, form.Info{
		Target: r.URL.Path, Method: "post", SubmitText: "save part",
	})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(errors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
	}

	page := template.Must(template.New("part").Funcs(renderer.FuncMap()).Parse(
		`{{ form_render .Form .Errors }}`,
	))
	if err := page.Execute(w, map[string]any{
		"Form": model, "Errors": errors,
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
		loggedIn := false
		if cookie, err := r.Cookie("sid"); err == nil {
			sessionsMu.Lock()
			_, loggedIn = sessions[cookie.Value]
			sessionsMu.Unlock()
		}

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
	log.Println(r.URL.Path, r.Method, r.RemoteAddr)
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

func submitOrder(recordOrder func(string, ...any) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := sessionID(w, r)

		cartsMu.Lock()
		cart := carts[id]
		if len(cart) == 0 {
			cartsMu.Unlock()
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		var parts string
		for _, item := range cart {
			parts += formatPart(item) + "\n"
		}
		cartsMu.Unlock()

		sessionsMu.Lock()
		username := sessions[id]
		sessionsMu.Unlock()
		orderID := rand.Text()
		if err := recordOrder(
			"%s: submitted by %q: %q",
			orderID,
			username,
			strings.TrimSuffix(parts, "\n"),
		); err != nil {
			log.Printf("ERROR %s: %q", orderID, err.Error())
		}
		auth := smtp.PlainAuth("", senderMail, senderPass, smtpServer)
		err := smtp.SendMail(
			smtpServer+":"+smtpPort,
			auth,
			senderMail,
			to,
			[]byte("subject: order alert\n\n"+parts+"\nrequester: "+username),
		)
		status := "sent"
		if err != nil {
			log.Printf("ERROR %s: %q", orderID, err.Error())
			status = fmt.Sprintf("failed: %v", err)
		}
		if logErr := recordOrder("%s: email %q", orderID, status); logErr != nil {
			log.Printf("ERROR %s: email %q: %q", orderID, status, logErr.Error())
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cartsMu.Lock()
		delete(carts, id)
		cartsMu.Unlock()

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
	http.HandleFunc("POST /submit", requireLogin(submitOrder(recordOrder)))
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
