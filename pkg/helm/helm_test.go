package helm

import (
	"os"
	"testing"
)

func TestGetCertManagerIssuerData(t *testing.T) {
	if getCertManagerIssuerData() != `apiVersion: certmanager.k8s.io/v1alpha1
kind: ClusterIssuer
metadata:
  name: localhost
spec:
  acme:
    server: https://acme-staging.api.letsencrypt.org/directory
    email: change_it@choerodon.io 
    privateKeySecretRef:
      name: localhost
    http01: {}
---
apiVersion: certmanager.k8s.io/v1alpha1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: change_it@choerodon.io 
    privateKeySecretRef:
      name: letsencrypt-prod
    http01: {}` {
		t.Fatal("format 01 cluster issuers incorrect")
	}

	err := os.Setenv("ACME_EMAIL", "test@c7n.co")
	if err == nil && getCertManagerIssuerData() != `apiVersion: certmanager.k8s.io/v1alpha1
kind: ClusterIssuer
metadata:
  name: localhost
spec:
  acme:
    server: https://acme-staging.api.letsencrypt.org/directory
    email: test@c7n.co 
    privateKeySecretRef:
      name: localhost
    http01: {}
---
apiVersion: certmanager.k8s.io/v1alpha1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: test@c7n.co 
    privateKeySecretRef:
      name: letsencrypt-prod
    http01: {}` {
		t.Fatal("format 02 cluster issuers incorrect")
	}
	os.Unsetenv("ACME_EMAIL")
}
