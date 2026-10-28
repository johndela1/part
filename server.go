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
	"strings"
	"sync"

	"github.com/donseba/go-form/v2"
)

var (
	sessions   = make(map[string]string)
	sessionsMu sync.Mutex
	carts      = make(map[string]*Cart)
	cartsMu    sync.Mutex

	to = []string{"glennwarden@yahoo.com", "john@jjdev.com"}

	// to = []string{"john@jjdev.com"}
	senderPass string = os.Getenv("SMTPPASS")
)

const (
	smtpServer = "smtp.gmail.com"
	smtpPort   = "587"
	senderMail = "john@jjdev.com"
)

type Cart struct {
	Items []any
}

func addToCart(id string, part any) {
	cartsMu.Lock()
	defer cartsMu.Unlock()

	if carts[id] == nil {
		carts[id] = &Cart{}
	}
	carts[id].Items = append(carts[id].Items, part)
}
func (o order) sendMail(msg string, w http.ResponseWriter) error {
	auth := smtp.PlainAuth("", senderMail, senderPass, smtpServer)
	msg += "\nrequester: " + o.requester
	err := smtp.SendMail(smtpServer+":"+smtpPort, auth, senderMail, to, []byte(msg))
	if err != nil {
		return err
	}

	successPage := `
		{{define "content"}}
			<h1>Sent</h1>
			<a href="/">&larr; Back</a>
		{{end}}`
	const baseTemplate = `
			<!DOCTYPE html>
			<html>
			<head>
			    <title>pot part ordering tool</title>
			</head>
			<body>
			    <div>
				{{template "content" .}}
			    </div>
			</body>
			</html>
			`
	tmpl := template.Must(template.New("layout").Parse(baseTemplate))
	tmpl = template.Must(tmpl.Parse(successPage))
	tmpl.Execute(w, nil)
	return nil
}

type Nipple struct {
	Size     string `form:"dropdown" label:"size" values:"close:close;1x2:1x2;1x2½:1x2½;1x3:1x3;1x3½:1x3½;1x4:1x4;1x4½:1x4½;1X5:1X5;1x5½:1x5½;1x6:1x6;1x6½:1x6½;1x7:1x7;1x7½:1x7½;1x8:1x8;1x8½:1x8½;1x9:1x9;1x9½:1x9½;1x10:1x10;1x10½:1x10½;1x11:1x11;1x11½:1x11½;1x12:1x12;" required:"true"`
	Quantity int    `form:"input,number" label:"Quantity" required:"true"`
}

type Pipe struct {
	ThicknessType string `form:"input,text" label:"Thickness Type" values:"SCH 10:SCH 10;SCH 40:SCH 40"`
	CoatingType   string `form:"input,text" label:"Coating Type" values:"Regular:Regular;Galvanized:Galvanized"`
	EndType       string `form:"input,text" label:"End Type" values:"Plain End:Plain End;Threaded:Threaded;Grooved:Grooved"`
	Size          string `form:"input,text" label:"Size" values:"8”:8”;6”:6”;4”:4”;3”:3”;2½”:2½”;2”:2”;1½”:1½”;1¼”:1¼”;1”:1”"`
	Quantity      int    `form:"input,number" label:"Quantity" required:"true"`
}

func partForm[T any](w http.ResponseWriter, r *http.Request) {
	var part T
	log.Println(r.URL.Path, r.Method, r.RemoteAddr)
	if r.Method == http.MethodPost {
		if err := form.MapForm(r, &part); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		log.Printf("post %+v", part)
		id, err := sessionID(w, r)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		addToCart(id, part)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	model := form.WithRequestInfo(r, part, form.Info{
		Target: r.URL.Path, Method: "post", SubmitText: "Save part",
	})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	renderer := form.NewForm()
	renderer.SetTheme("plain")
	page := template.Must(template.New("part").Funcs(renderer.FuncMap()).Parse(
		`{{ form_render .Form .Errors }}`,
	))
	if err := page.Execute(w, map[string]any{
		"Form": model, "Errors": form.FieldErrors(nil),
	}); err != nil {
		log.Println(err)
	}
}

type order struct {
	requester string
	items     []string
}

func readUsers(r io.Reader) (map[string]string, error) {
	users := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "|", 2)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid passwd entry: %q", scanner.Text())
		}
		users[fields[0]] = fields[1]
	}
	return users, scanner.Err()
}
func requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("sid")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		sessionsMu.Lock()
		_, ok := sessions[cookie.Value]
		sessionsMu.Unlock()

		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
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
	http.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		log.Println(r.URL.Path, r.Method, r.RemoteAddr)
		fmt.Fprint(w, `<form method="post" action="/login">
	<input name="username" placeholder="username" required autofocus><br>
        <input name="password" type="password" placeholder="password" required><br>
        <button type="submit">Login</button>
    </form>`)
	})

	http.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path, r.Method, r.RemoteAddr)
		username := r.FormValue("username")
		if password, ok := users[username]; !ok || password != r.FormValue("password") {
			http.Error(w, "Invalid login", http.StatusUnauthorized)
			return
		}

		id, err := sessionID(w, r)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		sessionsMu.Lock()
		sessions[id] = username
		sessionsMu.Unlock()

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	http.HandleFunc("GET /nipple", requireLogin(partForm[Nipple]))
	http.HandleFunc("POST /nipple", requireLogin(partForm[Nipple]))
	http.HandleFunc("GET /pipe", requireLogin(partForm[Pipe]))
	http.HandleFunc("POST /pipe", requireLogin(partForm[Pipe]))

	http.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `
			<!DOCTYPE html>
			<html>
			<body>
				<h1>
				<a href="./nipple">nipple</a><br>
				<a href="./pipe">pipe</a><br>
				</h1>
			</body>
			</html>
		`)
	})
	http.HandleFunc("GET /{$}", requireLogin(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path, r.Method, r.RemoteAddr)
		id, err := sessionID(w, r)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		cartsMu.Lock()
		var items []any
		if cart := carts[id]; cart != nil {
			items = append(items, cart.Items...)
		}
		cartsMu.Unlock()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		fmt.Fprint(w, `
			<!DOCTYPE html>
			<html>
			<body>
				<h1>
				<a href="./nipple">nipple</a><br>
				<a href="./pipe">pipe</a><br>
				</h1>
		`)
		for _, item := range items {
			switch p := item.(type) {
			case Pipe:
				fmt.Fprintf(w, "<p>pipe: %s</p>", template.HTMLEscapeString(fmt.Sprintf("%+v", p)))
			case Nipple:
				fmt.Fprintf(w, "<p>nipple: %s</p>", template.HTMLEscapeString(fmt.Sprintf("%+v", p)))
			}
		}

		if len(items) > 0 {
			fmt.Fprint(w, `
			<form action="/submit" method="post">
			    <button type="submit">Submit Order</button>
			</form>
		</body>
		</html>

		`)
		}
	}))
	http.HandleFunc("POST /submit", requireLogin(func(w http.ResponseWriter, r *http.Request) {
		id, err := sessionID(w, r)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		cartsMu.Lock()
		cart := carts[id]
		if cart == nil || len(cart.Items) == 0 {
			cartsMu.Unlock()
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		var parts string
		for _, item := range cart.Items {
			switch p := item.(type) {
			case Pipe:
				parts += fmt.Sprintf("Pipe: %+v\n", p)
			case Nipple:
				parts += fmt.Sprintf("Nipple: %+v\n", p)
			}
		}
		cartsMu.Unlock()

		sessionsMu.Lock()
		username := sessions[id]
		sessionsMu.Unlock()
		order := order{requester: username}
		orderID := rand.Text()
		if err := recordOrder("%s: submitted by %q: %q", orderID, username, strings.TrimSuffix(parts, "\n")); err != nil {
			log.Printf("ERROR %s: %q", orderID, err.Error())
		}
		err = order.sendMail("subject: order alert\n\n"+parts, w)
		status := "sent"
		if err != nil {
			log.Printf("ERROR %s: %q", orderID, err.Error())
			status = fmt.Sprintf("failed: %v", err)
		}
		if logErr := recordOrder("%s: email %q", orderID, status); logErr != nil {
			log.Printf("ERROR %s: email %q: %q", orderID, status, logErr.Error())
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		cartsMu.Lock()
		delete(carts, id)
		cartsMu.Unlock()
	}))
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

func sessionID(w http.ResponseWriter, r *http.Request) (string, error) {
	if c, err := r.Cookie("sid"); err == nil {
		return c.Value, nil
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	id := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name: "sid", Value: id, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	return id, nil
}
