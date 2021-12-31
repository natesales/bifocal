package main

import (
	"fmt"
	"net/smtp"
	"time"

	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	Username string `envconfig:"SSH_USERNAME" required:"true"`
	SSHKey   string `envconfig:"SSH_KEY" required:"true"`

	SMTPHost       string `envconfig:"SMTP_HOST" required:"true"`
	SMTPPort       int    `envconfig:"SMTP_PORT" required:"true"`
	SMTPUsername   string `envconfig:"SMTP_USERNAME" required:"true"`
	SMTPPassword   string `envconfig:"SMTP_PASSWORD" required:"true"`
	EmailRecipient string `envconfig:"EMAIL_RECIPIENT" required:"true"`

	Script   string `envconfig:"SCRIPT" required:"true"`
	Interval string `envconfig:"INTERVAL" required:"true"`
	Verbose  bool   `envconfig:"VERBOSE"`
}

// Linker flags
var version = "dev"

func main() {
	var c Config
	err := envconfig.Process("BIFOCAL", &c)
	if err != nil {
		log.Fatal(err)
	}

	if c.Verbose || version == "dev" {
		log.SetLevel(log.DebugLevel)
	}

	interval, err := time.ParseDuration(c.Interval)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Running bifocal every %s with script %s", interval, c.Script)

	conn, err := newConnector(c.Username, c.SSHKey)
	if err != nil {
		log.Fatal(err)
	}

	var (
		nodes        []Node
		totalQueries = 0
	)

	// Update nodes list every 24 hours
	ringQueryTicker := time.NewTicker(24 * time.Hour)
	go func() {
		for ; true; <-ringQueryTicker.C { // Tick once at start
			var err error
			nodes, err = ringNodes()
			if err != nil {
				log.Warn(err)
			} else {
				log.Infof("Retreived %d nodes", len(nodes))
			}

			// Log total queries
			log.Infof("%d total queries in the last 24 hours", totalQueries)
			totalQueries = 0
		}
	}()

	// Wait for ring node query to finish
	log.Info("Waiting for ring node cache")
	for {
		if len(nodes) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Run query on queryInterval
	queryTicker := time.NewTicker(interval)
	for ; true; <-queryTicker.C { // Tick once at start
		randNode, err := randomNode(nodes, 100)
		if err != nil {
			log.Warn(err)
			continue
		}

		log.Debugf("[%s] Connecting", randNode.Hostname)
		client, err := conn.connect(randNode.Hostname)
		if err != nil {
			log.Warnf("[%s] Unable to connect: %s", randNode.Hostname, err)
			continue
		}

		log.Debugf("[%s] Running query", randNode.Hostname)
		out, err := exec(client, fmt.Sprintf("sh -c 'curl -sL %s | bash'", c.Script))
		if err != nil || out != "" {
			log.Debugf("[%s] Query failed, sending email", randNode.Hostname)
			if err := smtp.SendMail(
				fmt.Sprintf("%s:%d", c.SMTPHost, c.SMTPPort),
				smtp.PlainAuth("", c.SMTPUsername, c.SMTPPassword, c.SMTPHost),
				c.SMTPUsername,
				[]string{c.EmailRecipient},
				[]byte(fmt.Sprintf(`To: "%s" <%s>
From: "%s" <%s>
Subject: Bifocal Alert

%s at %s

%s`,
					c.EmailRecipient, c.EmailRecipient, c.SMTPUsername, c.SMTPUsername, randNode.Hostname, time.Now().UTC(), out,
				)),
			); err != nil {
				log.Warnf("sending email: %s", err)
			}

			continue
		} else {
			log.Debugf("[%s] Query OK", randNode.Hostname)
		}

		totalQueries++
		client.Close()
	}
}
