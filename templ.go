package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/smtp"
	"os"

	"github.com/donseba/go-form/v2"
)

const (
	smtpServer = "smtp.gmail.com"
	smtpPort   = "587"
	senderMail = "john@jjdev.com"
)

// var to = []string{"glennwarden@yahoo.com", "john@jjdev.com"}
var to = []string{"john@jjdev.com"}
var senderPass string = os.Getenv("SMTPPASS")

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
			    <title>Part Ordering System</title>
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
	Size     string `form:"input,text" label:"size" values:"close:close;1x2:1x2;1x2 1/2:1x2 1/2;1x3:1x3;1x3 1/2:1x3 1/2;1x4:1x4;1x4 1/2:1x4 1/2;1X5:1X5;1x5 1/2:1x5 1/2;1x6:1x6;1x6 1/2:1x6 1/2;1x7:1x7;1x7 1/2:1x7 1/2;1x8:1x8;1x8 1/2:1x8 1/2;1x9:1x9;1x9 1/2:1x9 1/2;1x10:1x10;1x10 1/2:1x10 1/2;1x11:1x11;1x11 1/2:1x11 1/2;1x12:1x12;" required:"true"`
	Quantity int    `form:"input,number" label:"Quantity" required:"true"`
}

type Pipe struct {
	ThicknessType string `form:"input,text" label:"Thickness Type" values:"SCH 10:SCH 10;SCH 40:SCH 40"`
	CoatingType   string `form:"input,text" label:"Coating Type" values:"Regular:Regular;Galvanized:Galvanized"`
	EndType       string `form:"input,text" label:"End Type" values:"Plain End:Plain End;Threaded:Threaded;Grooved:Grooved"`
	Size          string `form:"input,text" label:"Size" values:"8”:8”;6”:6”;4”:4”;3”:3”;2 1/2”:2 1/2”;2”:2”;1 1/2”:1 1/2”;1 1/4”:1 1/4”;1”:1”"`
	Quantity      int    `form:"input,number" label:"Quantity" required:"true"`
}

func (o order) pipe(w http.ResponseWriter, r *http.Request) {
	fmt.Println("pipe", r.RemoteAddr)
	var part Pipe
	if r.Method == http.MethodPost {
		if err := form.MapForm(r, &part); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		fmt.Printf("post %+v\n", part)
		msg := fmt.Sprintf("subject: order alert\n\npart name: pipe  Qty: %v\n, thickness type: %v\ncoating type: %v\nend type: %v\nsize: %v\n", part.Quantity, part.ThicknessType, part.CoatingType, part.EndType, part.Size)
		err := o.sendMail(msg, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	model := form.WithRequestInfo(r, part, form.Info{
		Target: "/pipe", Method: "post", SubmitText: "Save part",
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

func (o order) nipple(w http.ResponseWriter, r *http.Request) {
	fmt.Println("nipple", r.RemoteAddr)
	var part Nipple
	if r.Method == http.MethodPost {
		if err := form.MapForm(r, &part); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		fmt.Printf("%+v\n", part)
		fmt.Fprintln(w, "XXX send mail!!!")
		return
	}

	model := form.WithRequestInfo(r, part, form.Info{
		Target: "/nipple", Method: "post", SubmitText: "Save part",
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

func main() {
	order := order{requester: "bob"}
	http.HandleFunc("/nipple", order.nipple)
	http.HandleFunc("/pipe", order.pipe)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("index", r.RemoteAddr)
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
	log.Fatal(http.ListenAndServe(":8080", nil))
}
