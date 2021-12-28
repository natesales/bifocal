package main

import (
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	username   = flag.String("u", "", "SSH username")
	sshKeyFile = flag.String("k", "", "SSH private key")
)

var (
	digCommand  = "dig +time=5 +tries=1 +nsid CH id.server TXT @ns1v4.packetframe.com"
	mtrCommand  = "mtr -wz ns1v4.packetframe.com"
	reNSID      = regexp.MustCompile(`; NSID.*`)
	reQueryTime = regexp.MustCompile(`;; Query time: (.*)`)
)

func main() {
	flag.Parse()

	conn, err := newConnector(*username, *sshKeyFile)
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

	// Run query every 5 minutes
	queryTicker := time.NewTicker(5 * time.Second)
	for ; true; <-queryTicker.C { // Tick once at start
		randNode, err := randomNode(nodes, 100)
		if err != nil {
			log.Warn(err)
			continue
		}

		log.Debugf("Connecting to %s", randNode.Hostname)
		client, err := conn.connect(randNode.Hostname)
		if err != nil {
			log.Warn(err)
			continue
		}

		dig, digErr := exec(client, digCommand)
		if digErr != nil {
			mtr, mtrErr := exec(client, mtrCommand)
			fmt.Printf(`
%s at %s

$ %s
%s
(%v)

$ %s
%s
(%v)`, randNode.Hostname, time.Now().UTC(),
				digCommand,
				dig,
				digErr,
				mtrCommand,
				mtr,
				mtrErr)
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
