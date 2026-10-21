package main

import (
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"sync"
)

const (
	smtpServer = "smtp.gmail.com"
	port       = "587"
	from       = "john@jjdev.com"
)
	var pass string      = os.Getenv("SMTPPASS")

type Order struct {
	ID        int
	Email     string
	PartName  string
	Quantity  int
	Fulfilled bool
}

// In-memory data store setup
var (
	orders         = make(map[int]Order)
	orderIDCounter = 1
	orderMutex     sync.Mutex
)

// Custom authentication structure to completely bypass standard string parsing bugs
type customPlainAuth struct {
	username, password, host string
}

func NewCustomPlainAuth(username, password, host string) smtp.Auth {
	return &customPlainAuth{username, password, host}
}

func (a *customPlainAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if server.Name != a.host {
		return "", nil, fmt.Errorf("wrong host name: %s", server.Name)
	}
	resp := []byte("\x00" + a.username + "\x00" + a.password)
	return "PLAIN", resp, nil
}

func (a *customPlainAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		return nil, fmt.Errorf("unexpected challenge from server")
	}
	return nil, nil
}

func main() {
	// 1. Customer form submission page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("req")
		w.Header().Set("Content-Type", "text/html")

		if r.Method == http.MethodPost {
			email := r.FormValue("email")
			partName := r.FormValue("part_name")
			qtyStr := r.FormValue("quantity")

			// SERVER-SIDE INPUT VALIDATION
			qty, err := strconv.Atoi(qtyStr)
			if err != nil || qty <= 0 || email == "" || partName == "" {
				http.Error(w, "Validation failed: Please check your form input fields.", http.StatusBadRequest)
				return
			}

			// CREATE AND INSERT ORDER RECORD
			orderMutex.Lock()
			newOrder := Order{
				ID:        orderIDCounter,
				Email:     email,
				PartName:  partName,
				Quantity:  qty,
				Fulfilled: false,
			}
			orders[newOrder.ID] = newOrder
			orderIDCounter++
			orderMutex.Unlock()

			// SEND ACKNOWLEDGMENT EMAIL PAYLOAD
			auth := NewCustomPlainAuth(from, pass, smtpServer)
			msg := []byte(fmt.Sprintf("Subject: Order #%d Received\n\nYour order for %d x %s has been successfully received.", newOrder.ID, qty, partName))
			addr := smtpServer + ":" + port

			if err := smtp.SendMail(addr, auth, from, []string{email}, msg); err != nil {
				http.Error(w, "Order record saved, but acknowledgment email failed: "+err.Error(), 500)
				return
			}

			fmt.Fprintf(w, "<h3>Order #%d created successfully and email sent!</h3><br><a href='/'>Submit another order</a> | <a href='/admin'>Go to Dashboard</a>", newOrder.ID)
			return
		}

		fmt.Fprint(w, `
			<h2>Order Parts</h2>
			<form method="POST">
				<input type="email" name="email" required placeholder="Your Email Address"><br><br>
				<input type="text" name="part_name" required placeholder="Part Name / Number"><br><br>
				<input type="number" name="quantity" required min="1" placeholder="Quantity"><br><br>
				<button type="submit">Submit Order</button>
			</form>
		`)
	})


	fmt.Println("Server running at http://0.0.0.0:8080")
	//http.ListenAndServe("0.0.0.0:8080", nil)
	listener, _ := net.Listen("tcp4", "0.0.0.0:8080") // "tcp4" forces IPv4 only
	http.Serve(listener, nil)
}
