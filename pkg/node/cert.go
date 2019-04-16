package node

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
	certv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeClient "k8s.io/client-go/kubernetes"

	"arhat.dev/aranya/pkg/constant"
)

func ensureNodeCert(client kubeClient.Interface, nodeObj *corev1.Node) (*tls.Certificate, error) {
	var (
		secretObjName = fmt.Sprintf("aranya.%s", nodeObj.Name)
		csrObjName    = fmt.Sprintf("aranya.%s", nodeObj.Name)

		needToCreateCSR      = false
		needToGetKubeCSR     = true
		needToCreateKubeCSR  = false
		needToApproveKubeCSR = true

		privateKeyBytes []byte
		csrBytes        []byte
		certBytes       []byte
	)

	secretClient := client.CoreV1().Secrets(constant.CurrentNamespace())
	pkSecret, err := secretClient.Get(secretObjName, metav1.GetOptions{})
	if err != nil {
		// create secret object if not found, add tls.csr
		if kubeErrors.IsNotFound(err) {
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

		var hosts []string
		for _, nodeAddr := range nodeObj.Status.Addresses {
			hosts = append(hosts, nodeAddr.Address)
		}

		csrBytes, err = csr.Generate(privateKey, &csr.CertificateRequest{
			CN: fmt.Sprintf("system:node:%s", nodeObj.Name),
			Names: []csr.Name{{
				O: "system:nodes",
			}},
			Hosts: hosts,
		})
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

		pkSecret, err = secretClient.Create(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretObjName,
				Namespace: constant.CurrentNamespace(),
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"tls.csr":               csrBytes,
				corev1.TLSPrivateKeyKey: privateKeyBytes,
				corev1.TLSCertKey:       []byte(""),
			},
		})
		if err != nil {
			log.Error(err, "failed create csr and private key secret")
			return nil, err
		}
	} else {
		var ok bool

		privateKeyBytes, ok = pkSecret.Data[corev1.TLSPrivateKeyKey]
		if !ok {
			return nil, errors.New("invalid csr and private secret, missing private key")
		}

		csrBytes, ok = pkSecret.Data["tls.csr"]
		if !ok {
			return nil, errors.New("invalid csr and private secret, missing csr")
		}

		certBytes, ok = pkSecret.Data[corev1.TLSCertKey]
		if ok && len(certBytes) > 0 {
			needToGetKubeCSR = false
		}
	}

	if needToGetKubeCSR {
		certClient := client.CertificatesV1beta1().CertificateSigningRequests()
		csrReq, err := certClient.Get(csrObjName, metav1.GetOptions{})
		if err != nil {
			if kubeErrors.IsNotFound(err) {
				needToCreateKubeCSR = true
			} else {
				log.Error(err, "failed to get certificate")
				return nil, err
			}
		}

		if needToCreateKubeCSR {
			csrObj := &certv1beta1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{
					// non namespaced
					Name: csrObjName,
				},
				Spec: certv1beta1.CertificateSigningRequestSpec{
					Request: csrBytes,
					Groups: []string{
						"system:nodes",
						"system:authenticated",
					},
					Usages: []certv1beta1.KeyUsage{
						certv1beta1.UsageServerAuth,
						certv1beta1.UsageDigitalSignature,
						certv1beta1.UsageKeyEncipherment,
					},
				},
			}

			csrReq, err = certClient.Create(csrObj)
			if err != nil {
				log.Error(err, "failed  to create csr")
				return nil, err
			}
		}

		for _, condition := range csrReq.Status.Conditions {
			switch condition.Type {
			case certv1beta1.CertificateApproved, certv1beta1.CertificateDenied:
				needToApproveKubeCSR = false
				break
			}
		}

		if needToApproveKubeCSR {
			csrReq.Status.Conditions = append(csrReq.Status.Conditions, certv1beta1.CertificateSigningRequestCondition{
				Type:    certv1beta1.CertificateApproved,
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
		log.Error(err, "failed to load certificate")
		return nil, err
	}

	return &cert, nil
}
