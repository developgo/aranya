// +build agent_grpc

/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"arhat.dev/aranya/pkg/connectivity/client"
	"arhat.dev/aranya/pkg/connectivity/client/runtime"
)

func New(arhatCtx context.Context, agentConfig *client.AgentConfig, clientConfig *client.ConnectivityConfig, rt runtime.Interface) (client.Interface, error) {
	grpcConfig := clientConfig.GRPCConfig
	if grpcConfig == nil {
		return nil, ErrConnectivityConfigNotProvided
	}

	dialCtx, cancel := context.WithTimeout(arhatCtx, grpcConfig.DialTimeout)
	defer cancel()

	dialOptions := []grpc.DialOption{grpc.WithBlock(), grpc.WithAuthority(grpcConfig.ServerAddress)}

	if tlsConfig := grpcConfig.TLS; tlsConfig != nil {
		if tlsConfig.ServerName == "" {
			colonPos := strings.LastIndex(grpcConfig.ServerAddress, ":")
			if colonPos == -1 {
				colonPos = len(grpcConfig.ServerAddress)
			}

			tlsConfig.ServerName = grpcConfig.ServerAddress[:colonPos]
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
				// use client side cert
				dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(certPool, tlsConfig.ServerName)))
			}
		} else {
			// use server side public cert
			dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, tlsCfg.ServerName)))
		}
	} else {
		dialOptions = append(dialOptions, grpc.WithInsecure())
	}

	conn, err := grpc.DialContext(dialCtx, grpcConfig.ServerAddress, dialOptions...)
	if err != nil {
		return nil, err
	}

	return client.NewGRPCAgent(arhatCtx, agentConfig, conn, rt)
}
