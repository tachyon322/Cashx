package outbox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"cashx/internal/platform"
)

// Dispatcher delivers a single outbox message. It must be idempotent-safe to
// retry (delivery is at-least-once).
type Dispatcher interface {
	Topic() string
	Dispatch(ctx context.Context, msg Message) error
}

// emailPayload is the JSON body of email.* outbox messages.
type emailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// EmailDispatcher sends mail via SMTP. Without CASHX_SMTP_HOST (dev) it logs
// the message and reports success so the queue drains.
type EmailDispatcher struct {
	cfg platform.Config
	log *slog.Logger
}

func NewEmailDispatcher(cfg platform.Config, log *slog.Logger) *EmailDispatcher {
	return &EmailDispatcher{cfg: cfg, log: log}
}

func (d *EmailDispatcher) Topic() string { return "email" }

func (d *EmailDispatcher) Dispatch(ctx context.Context, msg Message) error {
	var p emailPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return fmt.Errorf("bad email payload: %w", err)
	}
	if d.cfg.SMTPHost == "" {
		d.log.Info("email skipped (no SMTP configured)", "to", p.To, "subject", p.Subject)
		return nil
	}
	addr := net.JoinHostPort(d.cfg.SMTPHost, strconv.Itoa(d.cfg.SMTPPort))
	auth := smtp.PlainAuth("", d.cfg.SMTPUsername, d.cfg.SMTPPassword, d.cfg.SMTPHost)
	body := "From: " + d.cfg.SMTPFrom + "\r\n" +
		"To: " + p.To + "\r\n" +
		"Subject: " + p.Subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + p.Body
	done := make(chan error, 1)
	go func() {
		var err error
		if d.cfg.SMTPPort == 465 {
			err = smtp.SendMail(addr, nil, d.cfg.SMTPFrom, []string{p.To}, []byte(body))
		} else {
			client, cerr := smtp.Dial(addr)
			if cerr != nil {
				done <- cerr
				return
			}
			defer client.Close()
			if err = client.Hello("cashx"); err == nil {
				if d.cfg.SMTPUsername != "" {
					err = client.Auth(auth)
				}
			}
			if err == nil {
				err = client.StartTLS(&tls.Config{ServerName: d.cfg.SMTPHost})
			}
			if err == nil {
				err = client.Mail(d.cfg.SMTPFrom)
			}
			if err == nil {
				err = client.Rcpt(p.To)
			}
			if err == nil {
				w, werr := client.Data()
				if werr != nil {
					err = werr
				} else {
					_, err = w.Write([]byte(strings.ReplaceAll(body, "\r\n", "\n")))
					if err == nil {
						err = w.Close()
					}
				}
			}
			if err == nil {
				err = client.Quit()
			}
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("smtp send: %w", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(15 * time.Second):
		return fmt.Errorf("smtp send timed out")
	}
}

// TelegramDispatcher is a logging placeholder; Telegram integration is out of
// this milestone's scope and no message with topic "telegram" is enqueued yet.
type TelegramDispatcher struct {
	log *slog.Logger
}

func NewTelegramDispatcher(log *slog.Logger) *TelegramDispatcher {
	return &TelegramDispatcher{log: log}
}

func (d *TelegramDispatcher) Topic() string { return "telegram" }

func (d *TelegramDispatcher) Dispatch(ctx context.Context, msg Message) error {
	d.log.Warn("telegram dispatch not implemented; dropping message", "id", msg.ID)
	return nil
}

// Registry maps topic -> dispatcher.
type Registry map[string]Dispatcher

// NewRegistry builds the dispatcher set for the worker.
func NewRegistry(cfg platform.Config, log *slog.Logger) Registry {
	return Registry{
		"email":    NewEmailDispatcher(cfg, log),
		"telegram": NewTelegramDispatcher(log),
	}
}
