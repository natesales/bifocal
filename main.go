package main

import (
	"fmt"
	"net/smtp"
	"regexp"
	"strconv"
	"strings"
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

	Verbose bool `envconfig:"VERBOSE"`
}

var (
	// Linker flags
	version = "dev"

	queryInterval = 5 * time.Second
	target        = "ns1v4.packetframe.com"
	digCommand    = "dig +time=5 +tries=1 +nsid CH id.server TXT @" + target
	mtrCommand    = "mtr -wz " + target
	reNSID        = regexp.MustCompile(`; NSID.*`)
	reQueryTime   = regexp.MustCompile(`;; Query time: (.*)`)
)

func main() {
	var c Config
	err := envconfig.Process("BIFOCAL", &c)
	if err != nil {
		log.Fatal(err)
	}

	if c.Verbose || version == "dev" {
		log.SetLevel(log.DebugLevel)
	}

	conn, err := newConnector(c.Username, c.SSHKey)
	if err != nil {
		log.Fatal(err)
	}

	var nodes []Node

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
	queryTicker := time.NewTicker(queryInterval)
	for ; true; <-queryTicker.C { // Tick once at start
		randNode, err := randomNode(nodes, 100)
		if err != nil {
			log.Warn(err)
			continue
		}

		log.Debugf("[%s] Connecting", randNode.Hostname)
		client, err := conn.connect(randNode.Hostname)
		if err != nil {
			log.Warn(err)
			continue
		}

		log.Debugf("[%s] Running %s", randNode.Hostname, digCommand)
		dig, digErr := exec(client, digCommand)
		if digErr != nil {
			mtr, mtrErr := exec(client, mtrCommand)
			notifyMessage := fmt.Sprintf(`%s at %s

$ %s
%s
(%v)

$ %s
%s
(%v)
`, randNode.Hostname, time.Now().UTC(),
				digCommand,
				dig,
				digErr,
				mtrCommand,
				mtr,
				mtrErr)

			log.Info(notifyMessage)

			// Send notification email
			if err := smtp.SendMail(
				fmt.Sprintf("%s:%d", c.SMTPHost, c.SMTPPort),
				smtp.PlainAuth("", c.SMTPUsername, c.SMTPPassword, c.SMTPHost),
				c.SMTPUsername,
				[]string{c.EmailRecipient},
				[]byte(fmt.Sprintf(`To: "%s" <%s>
From: "%s" <%s>
Subject: Bifocal Alert

%s`,
					c.EmailRecipient, c.EmailRecipient, c.SMTPUsername, c.SMTPUsername, notifyMessage,
				)),
			); err != nil {
				log.Warnf("sending email: %s", err)
			}

			continue
		}

		nsid := strings.Split(string(reNSID.Find([]byte(dig))), "\"")[1]
		queryTime, err := strconv.Atoi(strings.Split(string(reQueryTime.Find([]byte(dig))), " ")[3])
		if err != nil {
			log.Warn(err)
			continue
		}

		log.Debugf("%s (%s) -> %s in %dms\n", randNode.Hostname, randNode.CountryCode, nsid, queryTime)

		client.Close()
	}
}
