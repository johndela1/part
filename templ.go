package main

import (
	"fmt"
	"html/template"
	"net/smtp"
	"log"
	"net/http"
	"os"

	"github.com/donseba/go-form/v2"
)

const (
	smtpServer   = "smtp.gmail.com"
	smtpPort     = "587"
	senderMail   = "john@jjdev.com"
	//to = "glennwarden@yahoo.com,john@jjdev.com"
)

var to = []string{"glennwarden@yahoo.com", "john@jjdev.com"}
var senderPass string = os.Getenv("SMTPPASS")

type Nipple struct {
	Size     string `form:"input,text" label:"size" values:"1:close; 2:1x2; 3:1x2 1/2; 4:1x3; 5:1x3 1/2; 6:1x4; 7:1x4 1/2; 8:1X5; 9:1x5 1/2; 10:1x6; 11:1x6 1/2; 12:1x7; 13:1x7 1/2; 14:1x8; 15:1x8 1/2; 16:1x9; 17:1x9 1/2; 18:1x10; 19:1x10 1/2; 20:1x11; 21:1x11 1/2; 22:1x12;" required:"true"`
	Quantity int    `form:"input,number" label:"Quantity" required:"true"`
}
type Pipe struct {
	ThicknessType string `form:"input,text" label:"Thickness Type" values:"1:SCH 10;2:SCH 40"`
	CoatingType   string `form:"input,text" label:"Coating Type" values:"1:Regular;2:Galvanized"`
	EndType       string `form:"input,text" label:"End Type" values:"1:Plain End;2:Threaded;3:Grooved"`
	Size          string `form:"input,text,text" label:"Size" values:"1:8”;2:6”;3:4”;4:3”;5:2 1/2”;6:2”;7:1  1/2”;8:1 1/4”;9:1”"`
	Quantity      int    `form:"input,number" label:"Quantity" required:"true"`
}

func pipe(w http.ResponseWriter, r *http.Request) {
	var part Pipe
	fmt.Println("hey")
	if r.Method == http.MethodPost {
	fmt.Println("now")
		if err := form.MapForm(r, &part); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		fmt.Printf("%+v\n", part)
		msg := fmt.Sprintf("Subject: Order Alert\n\nPart name: pipe  Qty: %v\n, thickness type: %v\ncoating type: %v\nend type: %v\nsize: %v\n", part.Quantity, part.ThicknessType,  part.CoatingType, part.EndType, part.Size)
		auth := smtp.PlainAuth("", senderMail, senderPass, smtpServer)
		err := smtp.SendMail(smtpServer+":"+smtpPort, auth, senderMail, to, []byte(msg))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		successPage := `{{define "content"}}
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
func nipple(w http.ResponseWriter, r *http.Request) {
	var part Nipple

	if r.Method == http.MethodPost {
		if err := form.MapForm(r, &part); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		fmt.Printf("%+v\n", part)
		fmt.Fprintln(w, "Submitted")
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

	http.HandleFunc("/nipple", nipple)
	http.HandleFunc("/pipe", pipe)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `
			<!DOCTYPE html>
			<html>
			<body>
				<a href="./nipple">nipple</a><br>
				<a href="./pipe">pipe</a><br>
			</body>
			</html>
		`)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
