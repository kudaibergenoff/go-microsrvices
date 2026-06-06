package smtp

import (
	"fmt"
	"net/smtp"
)

type Mailer struct {
	host string
	port string
	from string
}

func NewMailer(host, port, from string) *Mailer {
	return &Mailer{host: host, port: port, from: from}
}

func (m *Mailer) Send(to, subject, body string) error {
	addr := m.host + ":" + m.port

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		m.from, to, subject, body)

	return smtp.SendMail(addr, nil, m.from, []string{to}, []byte(msg))
}

func (m *Mailer) SendWelcome(to, name string) error {
	subject := "Welcome to Microservices!"
	body := fmt.Sprintf(`
		<h2>Welcome, %s!</h2>
		<p>Your account has been created successfully.</p>
		<p>Thank you for registering!</p>
	`, name)

	return m.Send(to, subject, body)
}
