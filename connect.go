package main

import (
	"encoding/pem"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type connector struct {
	User string
	Auth ssh.AuthMethod
}

func newConnector(user, privateKeyFile string) (*connector, error) {
	pemBytes, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, err
	}
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, err
	}
	return &connector{User: user, Auth: ssh.PublicKeys(signer)}, nil
}

// connect opens a SSH session to a host
func (c *connector) connect(host string) (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User: c.User,
		Auth: []ssh.AuthMethod{c.Auth},
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	return ssh.Dial("tcp", host+":22", sshConfig)
}

// exec executes a command on a remote machine
func exec(client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	log.Debugf("Running %s", command)
	out, err := session.CombinedOutput(command)
	return string(out), err
}
