// Author: recallsong
// Email: songruiguo@qq.com

package etcd

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/erda-project/erda-infra/base/logs"
	"github.com/erda-project/erda-infra/base/servicehub"
)

// Interface .
type Interface interface {
	Connect() (*clientv3.Client, error)
	Client() *clientv3.Client
	Timeout() time.Duration
}

type config struct {
	Endpoints string        `file:"endpoints" env:"ETCD_ENDPOINTS"`
	Timeout   time.Duration `file:"timeout" default:"10s"`
	TLS       struct {
		CertFile    string `file:"cert_file"`
		CertKeyFile string `file:"cert_key_file"`
		CaFile      string `file:"ca_file"`
	} `file:"tls"`
}

var clientType = reflect.TypeOf((*clientv3.Client)(nil))

type define struct{}

func (d *define) Services() []string { return []string{"etcd", "etcd-client"} }
func (d *define) Types() []reflect.Type {
	return []reflect.Type{
		reflect.TypeOf((*Interface)(nil)).Elem(),
		clientType,
	}
}
func (d *define) Description() string { return "etcd" }
func (d *define) Config() interface{} { return &config{} }
func (d *define) Creator() servicehub.Creator {
	return func() servicehub.Provider {
		return &provider{}
	}
}

type provider struct {
	Cfg       *config
	Log       logs.Logger
	client    *clientv3.Client
	tlsConfig *tls.Config
}

func (p *provider) Init(ctx servicehub.Context) error {
	err := p.initTLSConfig()
	if err != nil {
		return err
	}
	client, err := p.Connect()
	if err != nil {
		return err
	}
	p.client = client
	return nil
}

func (p *provider) Connect() (*clientv3.Client, error) {
	config := clientv3.Config{
		Endpoints:   strings.Split(p.Cfg.Endpoints, ","),
		DialTimeout: p.Cfg.Timeout,
		TLS:         p.tlsConfig,
	}
	return clientv3.New(config)
}

func (p *provider) Client() *clientv3.Client { return p.client }

func (p *provider) Timeout() time.Duration { return p.Cfg.Timeout }

func (p *provider) initTLSConfig() error {
	if len(p.Cfg.TLS.CertFile) > 0 || len(p.Cfg.TLS.CertKeyFile) > 0 {
		cfg, err := readTLSConfig(p.Cfg.TLS.CertFile, p.Cfg.TLS.CertKeyFile, p.Cfg.TLS.CaFile)
		if err != nil {
			if os.IsNotExist(err) {
				p.Log.Warnf("fail to load tls files: %s", err)
				return nil
			}
			return err
		}
		p.tlsConfig = cfg
	}
	return nil
}

func readTLSConfig(certFile, certKeyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, certKeyFile)
	if err != nil {
		return nil, err
	}
	caData, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caData)
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

// Provide .
func (p *provider) Provide(ctx servicehub.DependencyContext, args ...interface{}) interface{} {
	if ctx.Type() == clientType || ctx.Service() == "etcd-client" {
		return p.client
	}
	return p
}

func init() {
	servicehub.RegisterProvider("etcd", &define{})
}