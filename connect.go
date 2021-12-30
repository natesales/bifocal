package main

import (
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

type connector struct {
	User string
	Auth ssh.AuthMethod
}

func newConnector(user, privateKey string) (*connector, error) {
	pemBlock, _ := pem.Decode([]byte(privateKey))
	if pemBlock == nil {
		return nil, fmt.Errorf("unable to decode pem block: %s", privateKey)
	}
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
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

	out, err := session.CombinedOutput(command)
	return string(out), err
}
