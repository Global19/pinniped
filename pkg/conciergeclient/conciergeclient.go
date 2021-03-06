// Copyright 2020-2021 the Pinniped contributors. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package conciergeclient provides login helpers for the Pinniped concierge.
package conciergeclient

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthenticationv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	auth1alpha1 "go.pinniped.dev/generated/1.20/apis/concierge/authentication/v1alpha1"
	loginv1alpha1 "go.pinniped.dev/generated/1.20/apis/concierge/login/v1alpha1"
	conciergeclientset "go.pinniped.dev/generated/1.20/client/concierge/clientset/versioned"
	"go.pinniped.dev/internal/constable"
	"go.pinniped.dev/internal/groupsuffix"
	"go.pinniped.dev/internal/kubeclient"
)

// ErrLoginFailed is returned by Client.ExchangeToken when the concierge server rejects the login request for any reason.
var ErrLoginFailed = constable.Error("login failed")

// Option is an optional configuration for New().
type Option func(*Client) error

// Client is a configuration for talking to the Pinniped concierge.
type Client struct {
	namespace         string
	authenticatorName string
	authenticatorKind string
	caBundle          string
	endpoint          *url.URL
	apiGroupSuffix    string
}

// WithNamespace configures the namespace where the TokenCredentialRequest is to be sent.
func WithNamespace(namespace string) Option {
	return func(c *Client) error {
		c.namespace = namespace
		return nil
	}
}

// WithAuthenticator configures the authenticator reference (spec.authenticator) of the TokenCredentialRequests.
func WithAuthenticator(authType, authName string) Option {
	return func(c *Client) error {
		if authName == "" {
			return fmt.Errorf("authenticator name must not be empty")
		}
		c.authenticatorName = authName
		switch strings.ToLower(authType) {
		case "webhook":
			c.authenticatorKind = "WebhookAuthenticator"
		case "jwt":
			c.authenticatorKind = "JWTAuthenticator"
		default:
			return fmt.Errorf(`invalid authenticator type: %q, supported values are "webhook" and "jwt"`, authType)
		}
		return nil
	}
}

// WithCABundle configures the PEM-formatted TLS certificate authority to trust when connecting to the concierge.
func WithCABundle(caBundle string) Option {
	return func(c *Client) error {
		if caBundle == "" {
			return nil
		}
		if p := x509.NewCertPool(); !p.AppendCertsFromPEM([]byte(caBundle)) {
			return fmt.Errorf("invalid CA bundle data: no certificates found")
		}
		c.caBundle = caBundle
		return nil
	}
}

// WithBase64CABundle configures the base64-encoded, PEM-formatted TLS certificate authority to trust when connecting to the concierge.
func WithBase64CABundle(caBundleBase64 string) Option {
	return func(c *Client) error {
		caBundle, err := base64.StdEncoding.DecodeString(caBundleBase64)
		if err != nil {
			return fmt.Errorf("invalid CA bundle data: %w", err)
		}
		return WithCABundle(string(caBundle))(c)
	}
}

// WithEndpoint configures the base API endpoint URL of the concierge service (same as Kubernetes API server).
func WithEndpoint(endpoint string) Option {
	return func(c *Client) error {
		if endpoint == "" {
			return fmt.Errorf("endpoint must not be empty")
		}
		u, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("invalid endpoint URL: %w", err)
		}
		if u.Scheme != "https" {
			return fmt.Errorf(`invalid endpoint scheme %q (must be "https")`, u.Scheme)
		}
		c.endpoint = u
		return nil
	}
}

// WithAPIGroupSuffix configures the concierge's API group suffix (e.g., "pinniped.dev").
func WithAPIGroupSuffix(apiGroupSuffix string) Option {
	return func(c *Client) error {
		if err := groupsuffix.Validate(apiGroupSuffix); err != nil {
			return fmt.Errorf("invalid api group suffix: %w", err)
		}
		c.apiGroupSuffix = apiGroupSuffix
		return nil
	}
}

// New validates the specified options and returns a newly initialized *Client.
func New(opts ...Option) (*Client, error) {
	c := Client{namespace: "pinniped-concierge", apiGroupSuffix: "pinniped.dev"}
	for _, opt := range opts {
		if err := opt(&c); err != nil {
			return nil, err
		}
	}
	if c.authenticatorName == "" {
		return nil, fmt.Errorf("WithAuthenticator must be specified")
	}
	if c.endpoint == nil {
		return nil, fmt.Errorf("WithEndpoint must be specified")
	}
	return &c, nil
}

// clientset returns an anonymous client for the concierge API.
func (c *Client) clientset() (conciergeclientset.Interface, error) {
	cfg, err := clientcmd.NewNonInteractiveClientConfig(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster": {
				Server:                   c.endpoint.String(),
				CertificateAuthorityData: []byte(c.caBundle),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"current": {
				Cluster:  "cluster",
				AuthInfo: "client",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"client": {},
		},
	}, "current", &clientcmd.ConfigOverrides{}, nil).ClientConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubeclient.New(
		kubeclient.WithConfig(cfg),
		kubeclient.WithMiddleware(groupsuffix.New(c.apiGroupSuffix)),
	)
	if err != nil {
		return nil, err
	}
	return client.PinnipedConcierge, nil
}

// ExchangeToken performs a TokenCredentialRequest against the Pinniped concierge and returns the result as an ExecCredential.
func (c *Client) ExchangeToken(ctx context.Context, token string) (*clientauthenticationv1beta1.ExecCredential, error) {
	clientset, err := c.clientset()
	if err != nil {
		return nil, err
	}
	replacedAPIGroupName, _ := groupsuffix.Replace(auth1alpha1.SchemeGroupVersion.Group, c.apiGroupSuffix)
	resp, err := clientset.LoginV1alpha1().TokenCredentialRequests(c.namespace).Create(ctx, &loginv1alpha1.TokenCredentialRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.namespace,
		},
		Spec: loginv1alpha1.TokenCredentialRequestSpec{
			Token: token,
			Authenticator: v1.TypedLocalObjectReference{
				APIGroup: &replacedAPIGroupName,
				Kind:     c.authenticatorKind,
				Name:     c.authenticatorName,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not login: %w", err)
	}
	if resp.Status.Credential == nil || resp.Status.Message != nil {
		if resp.Status.Message != nil {
			return nil, fmt.Errorf("%w: %s", ErrLoginFailed, *resp.Status.Message)
		}
		return nil, fmt.Errorf("%w: unknown cause", ErrLoginFailed)
	}

	return &clientauthenticationv1beta1.ExecCredential{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ExecCredential",
			APIVersion: "client.authentication.k8s.io/v1beta1",
		},
		Status: &clientauthenticationv1beta1.ExecCredentialStatus{
			ExpirationTimestamp:   &resp.Status.Credential.ExpirationTimestamp,
			ClientCertificateData: resp.Status.Credential.ClientCertificateData,
			ClientKeyData:         resp.Status.Credential.ClientKeyData,
			Token:                 resp.Status.Credential.Token,
		},
	}, nil
}
