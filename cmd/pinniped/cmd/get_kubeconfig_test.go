// Copyright 2020 the Pinniped contributors. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	configv1alpha1 "go.pinniped.dev/generated/1.19/apis/config/v1alpha1"
	pinnipedclientset "go.pinniped.dev/generated/1.19/client/clientset/versioned"
	pinnipedfake "go.pinniped.dev/generated/1.19/client/clientset/versioned/fake"
	"go.pinniped.dev/internal/here"
)

var (
	knownGoodUsageForGetKubeConfig = here.Doc(`
		Usage:
		  get-kubeconfig [flags]

		Flags:
		  -h, --help                        help for get-kubeconfig
			  --kubeconfig string           Path to the kubeconfig file
			  --kubeconfig-context string   Kubeconfig context override
			  --pinniped-namespace string   Namespace in which Pinniped was installed (default "pinniped")
			  --token string                Credential to include in the resulting kubeconfig output (Required)

		`)

	knownGoodHelpForGetKubeConfig = here.Doc(`
		Print a kubeconfig for authenticating into a cluster via Pinniped.

		Requires admin-like access to the cluster using the current
		kubeconfig context in order to access Pinniped's metadata.
		The current kubeconfig is found similar to how kubectl finds it:
		using the value of the --kubeconfig option, or if that is not
		specified then from the value of the KUBECONFIG environment
		variable, or if that is not specified then it defaults to
		.kube/config in your home directory.

		Prints a kubeconfig which is suitable to access the cluster using
		Pinniped as the authentication mechanism. This kubeconfig output
		can be saved to a file and used with future kubectl commands, e.g.:
			pinniped get-kubeconfig --token $MY_TOKEN > $HOME/mycluster-kubeconfig
			kubectl --kubeconfig $HOME/mycluster-kubeconfig get pods

		Usage:
		  get-kubeconfig [flags]

		Flags:
		  -h, --help                        help for get-kubeconfig
			  --kubeconfig string           Path to the kubeconfig file
			  --kubeconfig-context string   Kubeconfig context override
			  --pinniped-namespace string   Namespace in which Pinniped was installed (default "pinniped")
			  --token string                Credential to include in the resulting kubeconfig output (Required)
		`)
)

func TestNewGetKubeConfigCmd(t *testing.T) {
	spec.Run(t, "newGetKubeConfigCmd", func(t *testing.T, when spec.G, it spec.S) {
		var r *require.Assertions
		var stdout, stderr *bytes.Buffer

		it.Before(func() {
			r = require.New(t)

			stdout, stderr = bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{})
		})

		it("passes all flags to runFunc", func() {
			args := []string{
				"--token", "some-token",
				"--kubeconfig", "some-kubeconfig",
				"--kubeconfig-context", "some-kubeconfig-context",
				"--pinniped-namespace", "some-pinniped-namespace",
			}
			c := newGetKubeConfigCmd(args, stdout, stderr)

			runFuncCalled := false
			c.runFunc = func(
				out, err io.Writer,
				token, kubeconfigPathOverride, currentContextOverride, pinnipedInstallationNamespace string,
			) {
				runFuncCalled = true
				r.Equal("some-token", token)
				r.Equal("some-kubeconfig", kubeconfigPathOverride)
				r.Equal("some-kubeconfig-context", currentContextOverride)
				r.Equal("some-pinniped-namespace", pinnipedInstallationNamespace)
			}

			r.NoError(c.cmd.Execute())
			r.True(runFuncCalled)
			r.Empty(stdout.String())
			r.Empty(stderr.String())
		})

		it("requires the 'token' flag", func() {
			args := []string{
				"--kubeconfig", "some-kubeconfig",
				"--kubeconfig-context", "some-kubeconfig-context",
				"--pinniped-namespace", "some-pinniped-namespace",
			}
			c := newGetKubeConfigCmd(args, stdout, stderr)

			runFuncCalled := false
			c.runFunc = func(
				out, err io.Writer,
				token, kubeconfigPathOverride, currentContextOverride, pinnipedInstallationNamespace string,
			) {
				runFuncCalled = true
			}

			errorMessage := `required flag(s) "token" not set`
			r.EqualError(c.cmd.Execute(), errorMessage)
			r.False(runFuncCalled)

			output := "Error: " + errorMessage + "\n" + knownGoodUsageForGetKubeConfig
			r.Equal(output, stdout.String())
			r.Empty(stderr.String())
		})

		it("defaults the flags correctly", func() {
			args := []string{
				"--token", "some-token",
			}
			c := newGetKubeConfigCmd(args, stdout, stderr)

			runFuncCalled := false
			c.runFunc = func(
				out, err io.Writer,
				token, kubeconfigPathOverride, currentContextOverride, pinnipedInstallationNamespace string,
			) {
				runFuncCalled = true
				r.Equal("some-token", token)
				r.Equal("", kubeconfigPathOverride)
				r.Equal("", currentContextOverride)
				r.Equal("pinniped", pinnipedInstallationNamespace)
			}

			r.NoError(c.cmd.Execute())
			r.True(runFuncCalled)
			r.Empty(stdout.String())
			r.Empty(stderr.String())
		})

		it("fails when args are passed", func() {
			args := []string{
				"--token", "some-token",
				"some-arg",
			}
			c := newGetKubeConfigCmd(args, stdout, stderr)

			runFuncCalled := false
			c.runFunc = func(
				out, err io.Writer,
				token, kubeconfigPathOverride, currentContextOverride, pinnipedInstallationNamespace string,
			) {
				runFuncCalled = true
			}

			errorMessage := `unknown command "some-arg" for "get-kubeconfig"`
			r.EqualError(c.cmd.Execute(), errorMessage)
			r.False(runFuncCalled)

			output := "Error: " + errorMessage + "\n" + knownGoodUsageForGetKubeConfig
			r.Equal(output, stdout.String())
			r.Empty(stderr.String())
		})

		it("prints a nice help message", func() {
			args := []string{
				"--help",
			}
			c := newGetKubeConfigCmd(args, stdout, stderr)

			runFuncCalled := false
			c.runFunc = func(
				out, err io.Writer,
				token, kubeconfigPathOverride, currentContextOverride, pinnipedInstallationNamespace string,
			) {
				runFuncCalled = true
			}

			r.NoError(c.cmd.Execute())
			r.False(runFuncCalled)
			r.Equal(knownGoodHelpForGetKubeConfig, stdout.String())
			r.Empty(stderr.String())
		})
	}, spec.Parallel(), spec.Report(report.Terminal{}))
}

func expectedKubeconfigYAML(
	clusterCAData,
	clusterServer,
	command,
	// nolint: unparam // Pass in the token even if it is always the same in practice
	token,
	pinnipedEndpoint,
	pinnipedCABundle,
	// nolint: unparam // Pass in the namespace even if it is always the same in practice
	namespace string,
) string {
	return here.Docf(`
		apiVersion: v1
		clusters:
		- cluster:
			certificate-authority-data: %s
			server: %s
		  name: pinniped-cluster
		contexts:
		- context:
			cluster: pinniped-cluster
			user: pinniped-user
		  name: pinniped-cluster
		current-context: pinniped-cluster
		kind: Config
		preferences: {}
		users:
		- name: pinniped-user
		  user:
			exec:
			  apiVersion: client.authentication.k8s.io/v1beta1
			  args:
			  - exchange-credential
			  command: %s
			  env:
			  - name: PINNIPED_K8S_API_ENDPOINT
				value: %s
			  - name: PINNIPED_CA_BUNDLE
				value: %s
			  - name: PINNIPED_NAMESPACE
			    value: %s
			  - name: PINNIPED_TOKEN
				value: %s
			  installHint: |-
				The Pinniped CLI is required to authenticate to the current cluster.
				For more information, please visit https://pinniped.dev
		`, clusterCAData, clusterServer, command, pinnipedEndpoint, pinnipedCABundle, namespace, token)
}

func newCredentialIssuerConfig(
	name,
	//nolint: unparam // Pass in the namespace even if it is always the same in practice
	namespace,
	server,
	certificateAuthorityData string,
) *configv1alpha1.CredentialIssuerConfig {
	return &configv1alpha1.CredentialIssuerConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CredentialIssuerConfig",
			APIVersion: configv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: configv1alpha1.CredentialIssuerConfigStatus{
			KubeConfigInfo: &configv1alpha1.CredentialIssuerConfigKubeConfigInfo{
				Server:                   server,
				CertificateAuthorityData: base64.StdEncoding.EncodeToString([]byte(certificateAuthorityData)),
			},
		},
	}
}

func TestGetKubeConfig(t *testing.T) {
	spec.Run(t, "cmd.getKubeConfig", func(t *testing.T, when spec.G, it spec.S) {
		var r *require.Assertions
		var outputBuffer *bytes.Buffer
		var warningsBuffer *bytes.Buffer
		var fullPathToSelf string
		var pinnipedClient *pinnipedfake.Clientset
		const installationNamespace = "some-namespace"

		it.Before(func() {
			r = require.New(t)

			outputBuffer = new(bytes.Buffer)
			warningsBuffer = new(bytes.Buffer)

			var err error
			fullPathToSelf, err = os.Executable()
			r.NoError(err)

			pinnipedClient = pinnipedfake.NewSimpleClientset()
		})

		when("the CredentialIssuerConfig is found on the cluster with a configuration that matches the existing kubeconfig", func() {
			it.Before(func() {
				r.NoError(pinnipedClient.Tracker().Add(
					newCredentialIssuerConfig(
						"some-cic-name",
						installationNamespace,
						"https://fake-server-url-value",
						"fake-certificate-authority-data-value",
					),
				))
			})

			it("writes the kubeconfig to the given writer", func() {
				kubeClientCreatorFuncWasCalled := false
				err := getKubeConfig(outputBuffer,
					warningsBuffer,
					"some-token",
					"./testdata/kubeconfig.yaml",
					"",
					installationNamespace,
					func(restConfig *rest.Config) (pinnipedclientset.Interface, error) {
						kubeClientCreatorFuncWasCalled = true
						r.Equal("https://fake-server-url-value", restConfig.Host)
						r.Equal("fake-certificate-authority-data-value", string(restConfig.CAData))
						return pinnipedClient, nil
					},
				)
				r.NoError(err)
				r.True(kubeClientCreatorFuncWasCalled)

				r.Empty(warningsBuffer.String())
				r.Equal(expectedKubeconfigYAML(
					base64.StdEncoding.EncodeToString([]byte("fake-certificate-authority-data-value")),
					"https://fake-server-url-value",
					fullPathToSelf,
					"some-token",
					"https://fake-server-url-value",
					"fake-certificate-authority-data-value",
					installationNamespace,
				), outputBuffer.String())
			})

			when("the currentContextOverride is used to specify a context other than the default context", func() {
				it.Before(func() {
					// update the Server and CertificateAuthorityData to make them match the other kubeconfig context
					r.NoError(pinnipedClient.Tracker().Update(
						schema.GroupVersionResource{
							Group:    configv1alpha1.GroupName,
							Version:  configv1alpha1.SchemeGroupVersion.Version,
							Resource: "credentialissuerconfigs",
						},
						newCredentialIssuerConfig(
							"some-cic-name",
							installationNamespace,
							"https://some-other-fake-server-url-value",
							"some-other-fake-certificate-authority-data-value",
						),
						installationNamespace,
					))
				})

				when("that context exists", func() {
					it("writes the kubeconfig to the given writer using the specified context", func() {
						kubeClientCreatorFuncWasCalled := false
						err := getKubeConfig(outputBuffer,
							warningsBuffer,
							"some-token",
							"./testdata/kubeconfig.yaml",
							"some-other-context",
							installationNamespace,
							func(restConfig *rest.Config) (pinnipedclientset.Interface, error) {
								kubeClientCreatorFuncWasCalled = true
								r.Equal("https://some-other-fake-server-url-value", restConfig.Host)
								r.Equal("some-other-fake-certificate-authority-data-value", string(restConfig.CAData))
								return pinnipedClient, nil
							},
						)
						r.NoError(err)
						r.True(kubeClientCreatorFuncWasCalled)

						r.Empty(warningsBuffer.String())
						r.Equal(expectedKubeconfigYAML(
							base64.StdEncoding.EncodeToString([]byte("some-other-fake-certificate-authority-data-value")),
							"https://some-other-fake-server-url-value",
							fullPathToSelf,
							"some-token",
							"https://some-other-fake-server-url-value",
							"some-other-fake-certificate-authority-data-value",
							installationNamespace,
						), outputBuffer.String())
					})
				})

				when("that context does not exist the in the current kubeconfig", func() {
					it("returns an error", func() {
						err := getKubeConfig(outputBuffer,
							warningsBuffer,
							"some-token",
							"./testdata/kubeconfig.yaml",
							"this-context-name-does-not-exist-in-kubeconfig.yaml",
							installationNamespace,
							func(restConfig *rest.Config) (pinnipedclientset.Interface, error) { return pinnipedClient, nil },
						)
						r.EqualError(err, `context "this-context-name-does-not-exist-in-kubeconfig.yaml" does not exist`)
						r.Empty(warningsBuffer.String())
						r.Empty(outputBuffer.String())
					})
				})
			})

			when("the token passed in is empty", func() {
				it("returns an error", func() {
					err := getKubeConfig(outputBuffer,
						warningsBuffer,
						"",
						"./testdata/kubeconfig.yaml",
						"",
						installationNamespace,
						func(restConfig *rest.Config) (pinnipedclientset.Interface, error) { return pinnipedClient, nil },
					)
					r.EqualError(err, "--token flag value cannot be empty")
					r.Empty(warningsBuffer.String())
					r.Empty(outputBuffer.String())
				})
			})

			when("the kubeconfig path passed refers to a file that does not exist", func() {
				it("returns an error", func() {
					err := getKubeConfig(outputBuffer,
						warningsBuffer,
						"some-token",
						"./testdata/this-file-does-not-exist.yaml",
						"",
						installationNamespace,
						func(restConfig *rest.Config) (pinnipedclientset.Interface, error) { return pinnipedClient, nil },
					)
					r.EqualError(err, "stat ./testdata/this-file-does-not-exist.yaml: no such file or directory")
					r.Empty(warningsBuffer.String())
					r.Empty(outputBuffer.String())
				})
			})

			when("the kubeconfig path parameter is empty", func() {
				it.Before(func() {
					// Note that this is technically polluting other parallel tests in this file, but other tests
					// are always specifying the kubeconfigPathOverride parameter, so they're not actually looking
					// at the value of this environment variable.
					r.NoError(os.Setenv("KUBECONFIG", "./testdata/kubeconfig.yaml"))
				})

				it.After(func() {
					r.NoError(os.Unsetenv("KUBECONFIG"))
				})

				it("falls back to using the KUBECONFIG env var to find the kubeconfig file", func() {
					kubeClientCreatorFuncWasCalled := false
					err := getKubeConfig(outputBuffer,
						warningsBuffer,
						"some-token",
						"",
						"",
						installationNamespace,
						func(restConfig *rest.Config) (pinnipedclientset.Interface, error) {
							kubeClientCreatorFuncWasCalled = true
							r.Equal("https://fake-server-url-value", restConfig.Host)
							r.Equal("fake-certificate-authority-data-value", string(restConfig.CAData))
							return pinnipedClient, nil
						},
					)
					r.NoError(err)
					r.True(kubeClientCreatorFuncWasCalled)

					r.Empty(warningsBuffer.String())
					r.Equal(expectedKubeconfigYAML(
						base64.StdEncoding.EncodeToString([]byte("fake-certificate-authority-data-value")),
						"https://fake-server-url-value",
						fullPathToSelf,
						"some-token",
						"https://fake-server-url-value",
						"fake-certificate-authority-data-value",
						installationNamespace,
					), outputBuffer.String())
				})
			})

			when("the wrong pinniped namespace is passed in", func() {
				it("returns an error", func() {
					kubeClientCreatorFuncWasCalled := false
					err := getKubeConfig(outputBuffer,
						warningsBuffer,
						"some-token",
						"./testdata/kubeconfig.yaml",
						"",
						"this-is-the-wrong-namespace",
						func(restConfig *rest.Config) (pinnipedclientset.Interface, error) {
							kubeClientCreatorFuncWasCalled = true
							r.Equal("https://fake-server-url-value", restConfig.Host)
							r.Equal("fake-certificate-authority-data-value", string(restConfig.CAData))
							return pinnipedClient, nil
						},
					)
					r.EqualError(err, `No CredentialIssuerConfig was found in namespace "this-is-the-wrong-namespace". Is Pinniped installed on this cluster in namespace "this-is-the-wrong-namespace"?`)
					r.True(kubeClientCreatorFuncWasCalled)
				})
			})

			when("there is more than one CredentialIssuerConfig is found on the cluster", func() {
				it.Before(func() {
					r.NoError(pinnipedClient.Tracker().Add(
						newCredentialIssuerConfig(
							"another-cic-name",
							installationNamespace,
							"https://fake-server-url-value",
							"fake-certificate-authority-data-value",
						),
					))
				})

				it("returns an error", func() {
					kubeClientCreatorFuncWasCalled := false
					err := getKubeConfig(outputBuffer,
						warningsBuffer,
						"some-token",
						"./testdata/kubeconfig.yaml",
						"",
						installationNamespace,
						func(restConfig *rest.Config) (pinnipedclientset.Interface, error) {
							kubeClientCreatorFuncWasCalled = true
							r.Equal("https://fake-server-url-value", restConfig.Host)
							r.Equal("fake-certificate-authority-data-value", string(restConfig.CAData))
							return pinnipedClient, nil
						},
					)
					r.EqualError(err, `More than one CredentialIssuerConfig was found in namespace "some-namespace"`)
					r.True(kubeClientCreatorFuncWasCalled)
				})
			})
		})

		when("the CredentialIssuerConfig is found on the cluster with a configuration that does not match the existing kubeconfig", func() {
			when("the Server doesn't match", func() {
				it.Before(func() {
					r.NoError(pinnipedClient.Tracker().Add(
						newCredentialIssuerConfig(
							"some-cic-name",
							installationNamespace,
							"non-matching-pinniped-server-url",
							"fake-certificate-authority-data-value",
						),
					))
				})

				it("writes the kubeconfig to the given writer using the values found in the local kubeconfig and issues a warning", func() {
					kubeClientCreatorFuncWasCalled := false
					err := getKubeConfig(outputBuffer,
						warningsBuffer,
						"some-token",
						"./testdata/kubeconfig.yaml",
						"",
						installationNamespace,
						func(restConfig *rest.Config) (pinnipedclientset.Interface, error) {
							kubeClientCreatorFuncWasCalled = true
							r.Equal("https://fake-server-url-value", restConfig.Host)
							r.Equal("fake-certificate-authority-data-value", string(restConfig.CAData))
							return pinnipedClient, nil
						},
					)
					r.NoError(err)
					r.True(kubeClientCreatorFuncWasCalled)

					r.Equal(
						"WARNING: Server and certificate authority did not match between local kubeconfig and Pinniped's CredentialIssuerConfig on the cluster. Using local kubeconfig values.\n",
						warningsBuffer.String(),
					)
					r.Equal(expectedKubeconfigYAML(
						base64.StdEncoding.EncodeToString([]byte("fake-certificate-authority-data-value")),
						"https://fake-server-url-value",
						fullPathToSelf,
						"some-token",
						"https://fake-server-url-value",
						"fake-certificate-authority-data-value",
						installationNamespace,
					), outputBuffer.String())
				})
			})

			when("the CA doesn't match", func() {
				it.Before(func() {
					r.NoError(pinnipedClient.Tracker().Add(
						newCredentialIssuerConfig(
							"some-cic-name",
							installationNamespace,
							"https://fake-server-url-value",
							"non-matching-certificate-authority-data-value",
						),
					))
				})

				it("writes the kubeconfig to the given writer using the values found in the local kubeconfig and issues a warning", func() {
					kubeClientCreatorFuncWasCalled := false
					err := getKubeConfig(outputBuffer,
						warningsBuffer,
						"some-token",
						"./testdata/kubeconfig.yaml",
						"",
						installationNamespace,
						func(restConfig *rest.Config) (pinnipedclientset.Interface, error) {
							kubeClientCreatorFuncWasCalled = true
							r.Equal("https://fake-server-url-value", restConfig.Host)
							r.Equal("fake-certificate-authority-data-value", string(restConfig.CAData))
							return pinnipedClient, nil
						},
					)
					r.NoError(err)
					r.True(kubeClientCreatorFuncWasCalled)

					r.Equal(
						"WARNING: Server and certificate authority did not match between local kubeconfig and Pinniped's CredentialIssuerConfig on the cluster. Using local kubeconfig values.\n",
						warningsBuffer.String(),
					)
					r.Equal(expectedKubeconfigYAML(
						base64.StdEncoding.EncodeToString([]byte("fake-certificate-authority-data-value")),
						"https://fake-server-url-value",
						fullPathToSelf,
						"some-token",
						"https://fake-server-url-value",
						"fake-certificate-authority-data-value",
						installationNamespace,
					), outputBuffer.String())
				})
			})
		})

		when("the CredentialIssuerConfig is found on the cluster with an empty KubeConfigInfo", func() {
			it.Before(func() {
				r.NoError(pinnipedClient.Tracker().Add(
					&configv1alpha1.CredentialIssuerConfig{
						TypeMeta: metav1.TypeMeta{
							Kind:       "CredentialIssuerConfig",
							APIVersion: configv1alpha1.SchemeGroupVersion.String(),
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pinniped-config",
							Namespace: installationNamespace,
						},
						Status: configv1alpha1.CredentialIssuerConfigStatus{},
					},
				))
			})

			it("returns an error", func() {
				kubeClientCreatorFuncWasCalled := false
				err := getKubeConfig(outputBuffer,
					warningsBuffer,
					"some-token",
					"./testdata/kubeconfig.yaml",
					"",
					installationNamespace,
					func(restConfig *rest.Config) (pinnipedclientset.Interface, error) {
						kubeClientCreatorFuncWasCalled = true
						r.Equal("https://fake-server-url-value", restConfig.Host)
						r.Equal("fake-certificate-authority-data-value", string(restConfig.CAData))
						return pinnipedClient, nil
					},
				)
				r.True(kubeClientCreatorFuncWasCalled)
				r.EqualError(err, `CredentialIssuerConfig "pinniped-config" was missing KubeConfigInfo`)
				r.Empty(warningsBuffer.String())
				r.Empty(outputBuffer.String())
			})
		})

		when("the CredentialIssuerConfig does not exist on the cluster", func() {
			it("returns an error", func() {
				kubeClientCreatorFuncWasCalled := false
				err := getKubeConfig(outputBuffer,
					warningsBuffer,
					"some-token",
					"./testdata/kubeconfig.yaml",
					"",
					installationNamespace,
					func(restConfig *rest.Config) (pinnipedclientset.Interface, error) {
						kubeClientCreatorFuncWasCalled = true
						r.Equal("https://fake-server-url-value", restConfig.Host)
						r.Equal("fake-certificate-authority-data-value", string(restConfig.CAData))
						return pinnipedClient, nil
					},
				)
				r.True(kubeClientCreatorFuncWasCalled)
				r.EqualError(err, `No CredentialIssuerConfig was found in namespace "some-namespace". Is Pinniped installed on this cluster in namespace "some-namespace"?`)
				r.Empty(warningsBuffer.String())
				r.Empty(outputBuffer.String())
			})
		})

		when("there is an error while getting the CredentialIssuerConfig from the cluster", func() {
			it("returns an error", func() {
				err := getKubeConfig(outputBuffer,
					warningsBuffer,
					"some-token",
					"./testdata/kubeconfig.yaml",
					"",
					installationNamespace,
					func(restConfig *rest.Config) (pinnipedclientset.Interface, error) {
						return nil, fmt.Errorf("some error getting CredentialIssuerConfig")
					},
				)
				r.EqualError(err, "some error getting CredentialIssuerConfig")
				r.Empty(warningsBuffer.String())
				r.Empty(outputBuffer.String())
			})
		})
	}, spec.Parallel(), spec.Report(report.Terminal{}))
}
