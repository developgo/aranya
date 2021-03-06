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

package edgedevice

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/cloudflare/cfssl/csr"
	cfsslhelpers "github.com/cloudflare/cfssl/helpers"
	certapi "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"

	"arhat.dev/aranya/pkg/constant"
)

func getKubeletServerCert(client kubeclient.Interface, hostNodeName, virtualNodeName string, csrName csr.Name, nodeAddresses []corev1.NodeAddress) (*tls.Certificate, error) {
	var (
		hosts                  []string
		requiredValidAddresses = make(map[string]struct{})
	)
	for _, nodeAddr := range nodeAddresses {
		hosts = append(hosts, nodeAddr.Address)
		requiredValidAddresses[nodeAddr.Address] = struct{}{}
	}

	var (
		// kubernetes secret name
		secretObjName = fmt.Sprintf("kubelet-tls.%s.%s", hostNodeName, virtualNodeName)
		// kubernetes csr name
		csrObjName = fmt.Sprintf("kubelet-tls.%s.%s", hostNodeName, virtualNodeName)
		certReq    = &csr.CertificateRequest{
			// CommonName is the RBAC role name, which must be `system:node:{nodeName}`
			CN: fmt.Sprintf("system:node:%s", virtualNodeName),
			Names: []csr.Name{
				// TODO: add or remove after debug
				// {O: "system:nodes"},
				csrName,
			},
			Hosts: hosts,
		}

		needToCreateCSR      = false
		needToGetKubeCSR     = true
		needToCreateKubeCSR  = false
		needToApproveKubeCSR = true

		privateKeyBytes []byte
		csrBytes        []byte
		certBytes       []byte
	)

	secretClient := client.CoreV1().Secrets(constant.WatchNamespace())
	pkSecret, err := secretClient.Get(secretObjName, metav1.GetOptions{})
	if err != nil {
		// create secret object if not found, add tls.csr
		if kubeerrors.IsNotFound(err) {
			needToCreateCSR = true
		} else {
			log.Error(err, "failed to get csr and private key secret")
			return nil, err
		}
	}

	if needToCreateCSR {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			log.Error(err, "failed to generate private key for node cert")
			return nil, err
		}

		// generate a PEM encoded csr
		csrBytes, err = csr.Generate(privateKey, certReq)
		if err != nil {
			log.Error(err, "failed to generate csr")
			return nil, err
		}

		b, err := x509.MarshalECPrivateKey(privateKey)
		if err != nil {
			log.Error(err, "failed to marshal elliptic curve private key")
			return nil, err
		}
		privateKeyBytes = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})

		pkSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: constant.WatchNamespace(),
				Name:      secretObjName,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"tls.csr":               csrBytes,
				corev1.TLSPrivateKeyKey: privateKeyBytes,
				corev1.TLSCertKey:       []byte(""),
			},
		}
		pkSecret, err = secretClient.Create(pkSecret)
		if err != nil {
			log.Error(err, "failed create csr and private key secret")
			return nil, err
		}
	} else {
		var ok bool

		privateKeyBytes, ok = pkSecret.Data[corev1.TLSPrivateKeyKey]
		if !ok {
			return nil, errors.New("invalid secret, missing private key")
		}

		certBytes, ok = pkSecret.Data[corev1.TLSCertKey]
		if ok && len(certBytes) > 0 {
			needToGetKubeCSR = false
		}

		csrBytes, ok = pkSecret.Data["tls.csr"]
		if !ok && needToGetKubeCSR {
			// csr not found, but still need to get kubernetes csr, conflict
			// this could happen when user has created the tls secret with
			// same name in advance, but the
			return nil, errors.New("invalid secret, missing csr")
		} else if ok {
			// check whether old csr is valid
			// the csr could be invalid if aranya's host node has changed
			// its address, which is possible for nodes with dynamic addresses
			log.Info("validating old csr")

			var oldCSR *x509.CertificateRequest
			oldCSR, _, err = cfsslhelpers.ParseCSR(csrBytes)
			if err != nil {
				log.Error(err, "failed to parse old csr")
				return nil, err
			}

			oldValidAddress := make(map[string]struct{})
			for _, ip := range oldCSR.IPAddresses {
				oldValidAddress[ip.String()] = struct{}{}
			}
			for _, dnsName := range oldCSR.DNSNames {
				oldValidAddress[dnsName] = struct{}{}
			}

			for requiredAddress := range requiredValidAddresses {
				if _, ok := oldValidAddress[requiredAddress]; !ok {
					needToGetKubeCSR = true
					needToCreateKubeCSR = true
					break
				}
			}

			if needToCreateKubeCSR {
				log.Info("old csr invalid, possible host node address change, recreating")

				key, err := cfsslhelpers.ParsePrivateKeyPEM(privateKeyBytes)
				if err != nil {
					log.Error(err, "failed to parse private key")
					return nil, err
				}

				csrBytes, err = csr.Generate(key, certReq)
				if err != nil {
					log.Error(err, "failed to generate csr")
					return nil, err
				}
			} else {
				log.Info("old csr valid")
			}
		}
	}

	if needToGetKubeCSR {
		certClient := client.CertificatesV1beta1().CertificateSigningRequests()

		kubeCSRNotFound := false
		csrReq, err := certClient.Get(csrObjName, metav1.GetOptions{})
		if err != nil {
			if kubeerrors.IsNotFound(err) {
				kubeCSRNotFound = true
				needToCreateKubeCSR = true
			} else {
				log.Error(err, "failed to get certificate")
				return nil, err
			}
		}

		if needToCreateKubeCSR {
			// delete first to make sure no invalid data exists
			err := certClient.Delete(csrObjName, metav1.NewDeleteOptions(0))
			if err != nil && !kubeerrors.IsNotFound(err) {
				return nil, err
			}

			csrObj := &certapi.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{
					// not namespaced
					Name: csrObjName,
				},
				Spec: certapi.CertificateSigningRequestSpec{
					Request: csrBytes,
					Groups: []string{
						"system:nodes",
						"system:authenticated",
					},
					Usages: []certapi.KeyUsage{
						certapi.UsageServerAuth,
						certapi.UsageDigitalSignature,
						certapi.UsageKeyEncipherment,
					},
				},
			}

			if !kubeCSRNotFound {
				err = certClient.Delete(csrObjName, metav1.NewDeleteOptions(0))
				if err != nil {
					log.Error(err, "failed to delete outdated kube csr")
					return nil, err
				}
			}

			log.Info("trying to create new kube csr")
			csrReq, err = certClient.Create(csrObj)
			if err != nil {
				log.Error(err, "failed to crate new kube csr")
				return nil, err
			}
		}

		for _, condition := range csrReq.Status.Conditions {
			switch condition.Type {
			case certapi.CertificateApproved, certapi.CertificateDenied:
				needToApproveKubeCSR = false
				break
			}
		}

		if needToApproveKubeCSR {
			csrReq.Status.Conditions = append(csrReq.Status.Conditions, certapi.CertificateSigningRequestCondition{
				Type:    certapi.CertificateApproved,
				Reason:  "AutoApproved",
				Message: "self approved by aranya node",
			})

			csrReq, err = certClient.UpdateApproval(csrReq)
			if err != nil {
				log.Error(err, "failed to approve csr")
				return nil, err
			}
		}

		certBytes = csrReq.Status.Certificate
		if len(certBytes) == 0 {
			// this could happen since certificate won't be issued immediately
			// good luck next time :)
			// TODO: should we watch?
			return nil, errors.New("certificate not issued")
		}

		pkSecret.Data[corev1.TLSCertKey] = certBytes
		pkSecret, err = secretClient.Update(pkSecret)
		if err != nil {
			log.Error(err, "failed to update node secret")
			return nil, err
		}
	}

	cert, err := tls.X509KeyPair(certBytes, privateKeyBytes)
	if err != nil {
		log.Info("failed to load certificate", "err", err.Error())
		if err := secretClient.Delete(secretObjName, metav1.NewDeleteOptions(0)); err != nil {
			log.Info("failed to delete bad certificate")
		}

		return nil, err
	}
	return &cert, nil
}
