// Copyright 2020 the Pinniped contributors. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "go.pinniped.dev/generated/1.19/apis/supervisor/config/v1alpha1"
)

const keyWith32Bytes = "0123456789abcdef0123456789abcdef"

func TestSymmetricSecretHelper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                     string
		secretUsage              SecretUsage
		wantSetOIDCProviderField func(*configv1alpha1.OIDCProvider) string
	}{
		{
			name:        "token signing key",
			secretUsage: SecretUsageTokenSigningKey,
			wantSetOIDCProviderField: func(op *configv1alpha1.OIDCProvider) string {
				return op.Status.Secrets.TokenSigningKey.Name
			},
		},
		{
			name:        "state signing key",
			secretUsage: SecretUsageStateSigningKey,
			wantSetOIDCProviderField: func(op *configv1alpha1.OIDCProvider) string {
				return op.Status.Secrets.StateSigningKey.Name
			},
		},
		{
			name:        "state encryption key",
			secretUsage: SecretUsageStateEncryptionKey,
			wantSetOIDCProviderField: func(op *configv1alpha1.OIDCProvider) string {
				return op.Status.Secrets.StateEncryptionKey.Name
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			labels := map[string]string{
				"some-label-key-1": "some-label-value-1",
				"some-label-key-2": "some-label-value-2",
			}
			randSource := strings.NewReader(keyWith32Bytes)
			var oidcProviderIssuerValue string
			var symmetricKeyValue []byte
			h := NewSymmetricSecretHelper(
				"some-name-prefix-",
				labels,
				randSource,
				test.secretUsage,
				func(oidcProviderIssuer string, symmetricKey []byte) {
					require.True(t, oidcProviderIssuer == "" && symmetricKeyValue == nil, "expected notify func not to have been called yet")
					oidcProviderIssuerValue = oidcProviderIssuer
					symmetricKeyValue = symmetricKey
				},
			)

			parent := &configv1alpha1.OIDCProvider{
				ObjectMeta: metav1.ObjectMeta{
					UID:       "some-uid",
					Namespace: "some-namespace",
				},
			}
			child, err := h.Generate(parent)
			require.NoError(t, err)
			require.Equal(t, child, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-name-prefix-some-uid",
					Namespace: "some-namespace",
					Labels:    labels,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(parent, schema.GroupVersionKind{
							Group:   configv1alpha1.SchemeGroupVersion.Group,
							Version: configv1alpha1.SchemeGroupVersion.Version,
							Kind:    "OIDCProvider",
						}),
					},
				},
				Type: "secrets.pinniped.dev/symmetric",
				Data: map[string][]byte{
					"key": []byte(keyWith32Bytes),
				},
			})

			require.True(t, h.IsValid(parent, child))

			h.ObserveActiveSecretAndUpdateParentOIDCProvider(parent, child)
			require.Equal(t, parent.Spec.Issuer, oidcProviderIssuerValue)
			require.Equal(t, child.Name, test.wantSetOIDCProviderField(parent))
			require.Equal(t, child.Data["key"], symmetricKeyValue)
		})
	}
}

func TestSymmetricSecretHelperIsValid(t *testing.T) {
	tests := []struct {
		name   string
		child  func(*corev1.Secret)
		parent func(*configv1alpha1.OIDCProvider)
		want   bool
	}{
		{
			name: "wrong type",
			child: func(s *corev1.Secret) {
				s.Type = "wrong"
			},
			want: false,
		},
		{
			name: "empty type",
			child: func(s *corev1.Secret) {
				s.Type = ""
			},
			want: false,
		},
		{
			name: "data key is too short",
			child: func(s *corev1.Secret) {
				s.Data["key"] = []byte("short")
			},
			want: false,
		},
		{
			name: "data key does not exist",
			child: func(s *corev1.Secret) {
				delete(s.Data, "key")
			},
			want: false,
		},
		{
			name: "child not owned by parent",
			parent: func(op *configv1alpha1.OIDCProvider) {
				op.UID = "wrong"
			},
			want: false,
		},
		{
			name: "happy path",
			want: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			h := NewSymmetricSecretHelper("none of these args matter", nil, nil, SecretUsageTokenSigningKey, nil)

			parent := &configv1alpha1.OIDCProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-parent-name",
					Namespace: "some-namespace",
					UID:       "some-parent-uid",
				},
			}
			child := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-name-prefix-some-uid",
					Namespace: "some-namespace",
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(parent, schema.GroupVersionKind{
							Group:   configv1alpha1.SchemeGroupVersion.Group,
							Version: configv1alpha1.SchemeGroupVersion.Version,
							Kind:    "OIDCProvider",
						}),
					},
				},
				Type: "secrets.pinniped.dev/symmetric",
				Data: map[string][]byte{
					"key": []byte(keyWith32Bytes),
				},
			}
			if test.child != nil {
				test.child(child)
			}
			if test.parent != nil {
				test.parent(parent)
			}

			require.Equalf(t, test.want, h.IsValid(parent, child), "child: %#v", child)
		})
	}
}
