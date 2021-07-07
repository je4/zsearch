package sshtunnel

/*
https://gist.github.com/svett/5d695dcc4cc6ad5dd275

*/

import (
	"fmt"
	"github.com/goph/emperror"
	"github.com/op/go-logging"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"net"
	"sync"
	"time"
)

type Endpoint struct {
	Host string
	Port int
}

type SourceDestination struct {
	Local  *Endpoint
	Remote *Endpoint
}

func (endpoint *Endpoint) String() string {
	return fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)
}

type SSHtunnel struct {
	server   *Endpoint
	tunnels  map[string]*SourceDestination
	quit     map[string]chan interface{}
	listener map[string]net.Listener
	config   *ssh.ClientConfig
	client   *ssh.Client
	log      *logging.Logger
	wg       sync.WaitGroup
}

func NewSSHTunnel(user, privateKey string, serverEndpoint *Endpoint, tunnels map[string]*SourceDestination, log *logging.Logger) (*SSHtunnel, error) {
	key, err := ioutil.ReadFile(privateKey)
	if err != nil {
		return nil, emperror.Wrapf(err, "Unable to read private key %s", privateKey)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, emperror.Wrapf(err, "Unable to parse private key")
	}

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	tunnel := &SSHtunnel{
		config:   sshConfig,
		server:   serverEndpoint,
		tunnels:  tunnels,
		quit:     make(map[string]chan interface{}),
		listener: make(map[string]net.Listener),
		log:      log,
	}

	return tunnel, nil
}

func (tunnel *SSHtunnel) String() string {
	str := fmt.Sprintf("%v@%v:%v",
		tunnel.config.User,
		tunnel.server.Host, tunnel.server.Port,
	)
	for name, srcdests := range tunnel.tunnels {
		str += fmt.Sprintf(" - ([%v] %v:%v -> %v:%v)",
			name,
			srcdests.Local.Host, srcdests.Local.Port,
			srcdests.Remote.Host, srcdests.Remote.Port,
		)
	}
	return str
}

func (tunnel *SSHtunnel) Close() {
	tunnel.client.Close()
	for key, listener := range tunnel.listener {
		if q, ok := tunnel.quit[key]; ok {
			close(q)
		}
		listener.Close()
	}
	tunnel.wg.Wait()
}

func (tunnel *SSHtunnel) Start() error {
	var err error
	tunnel.log.Info("starting ssh connection listener")

	tunnel.log.Infof("dialing ssh: %v", tunnel.String())
	tunnel.client, err = ssh.Dial("tcp", tunnel.server.String(), tunnel.config)
	if err != nil {
		return emperror.Wrapf(err, "server dial error to %v", tunnel.server.String())
	}

	for key, t := range tunnel.tunnels {
		tunnel.log.Infof("starting tunnel [%v] %v:%v -> %v:%v)",
			key,
			t.Local.Host, t.Local.Port,
			t.Remote.Host, t.Remote.Port,
		)
		tunnel.listener[key], err = net.Listen("tcp", t.Local.String())
		if err != nil {
			return emperror.Wrapf(err, "cannot start listener on %v", t.Local.String())
		}
		tunnel.quit[key] = make(chan interface{})

		go func(k string) {
			//defer tunnel.wg.Done()
			tunnel.log.Debugf("start accepting %v", k)
			conn, err := tunnel.listener[k].Accept()
			if err != nil {
				select {
				case <-tunnel.quit[k]:
					tunnel.log.Errorf("quit tunnel %v", k)
					return
				default:
					tunnel.log.Errorf("error accepting connection on %v", tunnel.tunnels[k].Local.String())
				}
			} else {
				tunnel.wg.Add(1)
				go func() {
					tunnel.log.Debugf("tunnel %v forward()", k)
					tunnel.forward(conn, tunnel.tunnels[k].Remote)
					tunnel.wg.Done()
					tunnel.log.Debugf("tunnel %v forward() done", k)
				}()
			}
		}(key)
	}
	time.Sleep(2 * time.Second)
	return nil
}

func (tunnel *SSHtunnel) forward(localConn net.Conn, endpoint *Endpoint) {
	var err error

	remoteConn, err := tunnel.client.Dial("tcp", endpoint.String())
	if err != nil {
		tunnel.log.Errorf("Remote dial error %v: %v", endpoint.String(), err)
		return
	}

	copyConn := func(writer, reader net.Conn) {
		defer writer.Close()
		defer reader.Close()

		_, err := io.Copy(writer, reader)
		if err != nil {
			tunnel.log.Errorf("io.Copy error %v: %v", endpoint.String(), err)
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		copyConn(localConn, remoteConn)
		wg.Done()
	}()
	go func() {
		copyConn(remoteConn, localConn)
		wg.Done()
	}()
	wg.Wait()
}
