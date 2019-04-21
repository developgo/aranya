// +build agent_grpc

package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"arhat.dev/aranya/pkg/virtualnode/agent"
	"arhat.dev/aranya/pkg/virtualnode/agent/runtime"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func New(arhatCtx context.Context, agentConfig *agent.Config, connectivityConfig *connectivity.Config, rt runtime.Interface) (agent.Interface, error) {
	dialCtx, cancel := context.WithTimeout(arhatCtx, connectivityConfig.Server.DialTimeout)
	defer cancel()

	dialOptions := []grpc.DialOption{grpc.WithBlock(), grpc.WithAuthority(connectivityConfig.Server.Address)}

	if connectivityConfig.Server.TLS != nil {
		tlsConfig := connectivityConfig.Server.TLS
		if tlsConfig.ServerName == "" {
			colonPos := strings.LastIndex(connectivityConfig.Server.Address, ":")
			if colonPos == -1 {
				colonPos = len(connectivityConfig.Server.Address)
			}

			tlsConfig.ServerName = connectivityConfig.Server.Address[:colonPos]
		}

		tlsCfg := &tls.Config{
			ServerName: tlsConfig.ServerName,
		}

		if tlsConfig.CaCert != "" {
			caPool := x509.NewCertPool()
			caBytes, err := ioutil.ReadFile(tlsConfig.CaCert)
			if err != nil {
				return nil, err
			}

			caPool.AppendCertsFromPEM(caBytes)
			tlsCfg.RootCAs = caPool
		}

		if tlsConfig.Cert != "" {
			if tlsConfig.Key != "" {
				cert, err := tls.LoadX509KeyPair(tlsConfig.Cert, tlsConfig.Key)
				if err != nil {
					return nil, err
				}
				tlsCfg.Certificates = []tls.Certificate{cert}
				dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
			} else {
				certPool := x509.NewCertPool()
				certBytes, err := ioutil.ReadFile(tlsConfig.Cert)
				if err != nil {
					return nil, err
				}

				if !certPool.AppendCertsFromPEM(certBytes) {
					panic("append cert failed")
				}

				dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(certPool, tlsConfig.ServerName)))
			}
		} else {
			// use server side public cert
			dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")))
		}
	} else {
		dialOptions = append(dialOptions, grpc.WithInsecure())
	}

	conn, err := grpc.DialContext(dialCtx, connectivityConfig.Server.Address, dialOptions...)
	if err != nil {
		return nil, err
	}

	return agent.NewGRPCAgent(arhatCtx, agentConfig, conn, rt)
}
