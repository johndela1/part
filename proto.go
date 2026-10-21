package main

import (
	"html/template"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
)

const (
	smtpServer = "smtp.gmail.com"
	smtpPort   = "587"
	senderMail = "john@jjdev.com"
	receiverMail   = "john@jjdev.com"
)

var senderPass string = os.Getenv("SMTPPASS")
/*
Pipe Nipples
1” diameter
Expressed 1X2
for 1” diameter
2” length

CLOSE
1x2
1x2 1/2
1x3
1x3 1/2
1x4
1x4 1/2
1X5
1x5 1/2
1x6
1x6 1/2
1x7
1x7 1/2
1x8
1x8 1/2
1x9
1x9 1/2
1x10
1x10 1/2
1x11
1x11 1/2
1x12
*/
type Part struct {
	ID          int
	Name        string
	Description string
	Domestic    bool
}
type Part2 struct {
	ID          int
	Name        string
	Description string
	Domestic    bool
}

var partsList = []Part{
	{ID: 1, Name: "Engine Valve Alpha", Description: "High-tolerance intake valve."},
	{ID: 2, Name: "Hydraulic Seal Beta", Description: "Reinforced rubber pressure seal."},
}

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

func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/order", orderHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	indexPage := `{{define "content"}}
		<h1>Available Parts Catalog</h1>
		<ul>
			{{range .}}
			<li><a href="/order?id={{.ID}}">{{.Name}}</a> — {{.Description}}</li>
			{{end}}
		</ul>
	{{end}}`
	tmpl := template.Must(template.New("layout").Parse(baseTemplate))
	tmpl = template.Must(tmpl.Parse(indexPage))
	tmpl.Execute(w, partsList)
}

func orderHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 || id > len(partsList) {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	selectedPart := partsList[id-1]

	if r.Method == http.MethodPost {
		qty := r.FormValue("quantity")
		employee := r.FormValue("employee_name")

		msg := "Subject: Order Alert\n\nPart name: " + selectedPart.Name + " Qty: " + qty + " By: " + employee

		auth := smtp.PlainAuth("", senderMail, senderPass, smtpServer)
		err := smtp.SendMail(smtpServer+":"+smtpPort, auth, senderMail, []string{receiverMail}, []byte(msg))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		successPage := `{{define "content"}}
			<h1>Sent</h1>
			<a href="/">&larr; Back</a>
		{{end}}`
		tmpl := template.Must(template.New("layout").Parse(baseTemplate))
		tmpl = template.Must(tmpl.Parse(successPage))
		tmpl.Execute(w, nil)
		return
	}

	formPage := `{{define "content"}}
		<h1>Order Form: {{.Name}}</h1>
		<p>{{.Description}}</p>
		<form action="/order?id={{.ID}}" method="POST">
			<label>Your Name:</label>
			<input type="text" name="employee_name" required>
			<label>Quantity Required:</label>
			<input type="number" name="quantity" required min="1" value="1">
			<button type="submit">Submit Order Request via Email</button>
		</form>
		<a href="/">&larr; Cancel</a>
	{{end}}`
	tmpl := template.Must(template.New("layout").Parse(baseTemplate))
	tmpl = template.Must(tmpl.Parse(formPage))
	tmpl.Execute(w, selectedPart)
}
