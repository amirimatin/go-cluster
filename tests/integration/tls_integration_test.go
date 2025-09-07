//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amirimatin/go-cluster/pkg/bootstrap"
	tlsx "github.com/amirimatin/go-cluster/pkg/security/tlsconfig"
	"github.com/amirimatin/go-cluster/pkg/transport"
	httpjson "github.com/amirimatin/go-cluster/pkg/transport/httpjson"
)

func TestTLS_ThreeNodes_StatusAndJoin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dir := t.TempDir()
	caCrt, _, srvCrt, srvKey, cliCrt, cliKey := mustMakeTestCerts(t, dir)

	n1, err := bootstrap.Run(ctx, bootstrap.Config{
		NodeID: "n1", RaftAddr: "127.0.0.1:9521", MemBind: "127.0.0.1:7946", MgmtAddr: "127.0.0.1:17946",
		DiscoveryKind: "static", Bootstrap: true,
		TLSEnable: true, TLSCA: caCrt, TLSCert: srvCrt, TLSKey: srvKey,
	})
	if err != nil {
		t.Fatalf("n1: %v", err)
	}
	defer n1.Close()

	n2, err := bootstrap.Run(ctx, bootstrap.Config{
		NodeID: "n2", RaftAddr: "127.0.0.1:9522", MemBind: "127.0.0.1:8946", MgmtAddr: "127.0.0.1:18946",
		DiscoveryKind: "static", SeedsCSV: "127.0.0.1:7946",
		TLSEnable: true, TLSCA: caCrt, TLSCert: srvCrt, TLSKey: srvKey,
	})
	if err != nil {
		t.Fatalf("n2: %v", err)
	}
	defer n2.Close()

	n3, err := bootstrap.Run(ctx, bootstrap.Config{
		NodeID: "n3", RaftAddr: "127.0.0.1:9523", MemBind: "127.0.0.1:9946", MgmtAddr: "127.0.0.1:19946",
		DiscoveryKind: "static", SeedsCSV: "127.0.0.1:7946",
		TLSEnable: true, TLSCA: caCrt, TLSCert: srvCrt, TLSKey: srvKey,
	})
	if err != nil {
		t.Fatalf("n3: %v", err)
	}
	defer n3.Close()

	topts := tlsx.Options{Enable: true, CAFile: caCrt, CertFile: cliCrt, KeyFile: cliKey}
	cliTLS, err := topts.Client()
	if err != nil {
		t.Fatalf("tls client: %v", err)
	}
	cli := httpjson.NewClient(3 * time.Second).UseTLS(cliTLS)

	waitUntil(t, 20*time.Second, func() error {
		s, err := fetchStatus(ctx, cli, "127.0.0.1:17946")
		if err != nil {
			return err
		}
		if !s.Healthy || s.LeaderID != "n1" {
			return errNotYet
		}
		return nil
	})

	joinCtx, cancelJoin := context.WithTimeout(ctx, 5*time.Second)
	if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17946", transport.JoinRequest{ID: "n2", RaftAddr: "127.0.0.1:9522"}); err != nil {
		cancelJoin()
		t.Fatalf("join n2: %v", err)
	}
	if _, err := cli.PostJoin(joinCtx, "127.0.0.1:17946", transport.JoinRequest{ID: "n3", RaftAddr: "127.0.0.1:9523"}); err != nil {
		cancelJoin()
		t.Fatalf("join n3: %v", err)
	}
	cancelJoin()
}

func mustMakeTestCerts(t *testing.T, dir string) (caCrt, caKey, srvCrt, srvKey, cliCrt, cliKey string) {
	t.Helper()
	caPriv, _ := rsa.GenerateKey(rand.Reader, 2048)
	caTpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "go-cluster-ca"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(48 * time.Hour), KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, IsCA: true, BasicConstraintsValid: true}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTpl, caTpl, &caPriv.PublicKey, caPriv)
	caCrt = filepath.Join(dir, "ca.crt")
	caKey = filepath.Join(dir, "ca.key")
	writePEM(t, caCrt, "CERTIFICATE", caDER)
	writePEM(t, caKey, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caPriv))

	makeLeaf := func(cn, crtName, keyName string, isClient bool) (string, string) {
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		tpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: cn}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment}
		if isClient {
			tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		} else {
			tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		}
		tpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		der, _ := x509.CreateCertificate(rand.Reader, tpl, caTpl, &priv.PublicKey, caPriv)
		crtPath := filepath.Join(dir, crtName)
		keyPath := filepath.Join(dir, keyName)
		writePEM(t, crtPath, "CERTIFICATE", der)
		writePEM(t, keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(priv))
		return crtPath, keyPath
	}

	srvCrt, srvKey = makeLeaf("go-cluster-server", "server.crt", "server.key", false)
	cliCrt, cliKey = makeLeaf("go-cluster-client", "client.crt", "client.key", true)
	return
}

func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: typ, Bytes: der}); err != nil {
		t.Fatalf("pem encode %s: %v", path, err)
	}
}
