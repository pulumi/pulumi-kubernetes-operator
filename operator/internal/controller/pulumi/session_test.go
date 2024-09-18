// Copyright 2021, Pulumi Corporation.  All rights reserved.

package pulumi

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/go-logr/logr/testr"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/shared"
	v1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/v1"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	secretName = "fake-secret"
	namespace  = "test"
)

type GitAuthTestSuite struct {
	suite.Suite
	f string
}

func (suite *GitAuthTestSuite) SetupTest() {
	f, err := ioutil.TempFile("", "")
	suite.NoError(err)
	defer f.Close()
	f.WriteString("super secret")
	suite.f = f.Name()
	os.Setenv("SECRET3", "so secret")
}

func (suite *GitAuthTestSuite) AfterTest() {
	if suite.f != "" {
		os.Remove(suite.f)
	}
	os.Unsetenv("SECRET3")
	suite.T().Log("Cleaned up")
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(GitAuthTestSuite))
}

func (suite *GitAuthTestSuite) TestSetupGitAuthWithSecrets() {
	t := suite.T()
	t.Skip() // https://github.com/pulumi/pulumi-kubernetes-operator/pull/658
	log := testr.New(t).WithValues("Request.Test", "TestSetupGitAuthWithSecrets")

	sshPrivateKey := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sshPrivateKey",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"sshPrivateKey": []byte("very secret key"),
		},
		Type: "Opaque",
	}
	sshPrivateKeyWithPassword := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sshPrivateKeyWithPassword",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"sshPrivateKey": []byte("very secret key"),
			"password":      []byte("moar secret password"),
		},
		Type: "Opaque",
	}
	accessToken := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "accessToken",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"accessToken": []byte("super secret access token"),
		},
		Type: "Opaque",
	}
	basicAuth := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basicAuth",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"username": []byte("not so secret username"),
			"password": []byte("very secret password"),
		},
		Type: "Opaque",
	}
	basicAuthWithoutPassword := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basicAuthWithoutPassword",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"username": []byte("not so secret username"),
		},
		Type: "Opaque",
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(sshPrivateKey, sshPrivateKeyWithPassword, accessToken, basicAuth, basicAuthWithoutPassword).
		Build()

	for _, test := range []struct {
		name     string
		gitAuth  *shared.GitAuthConfig
		expected *auto.GitAuth
		err      error
	}{
		{
			name: "InvalidSecretName",
			gitAuth: &shared.GitAuthConfig{
				SSHAuth: &shared.SSHAuth{
					SSHPrivateKey: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      "MISSING",
							},
						},
					},
				},
			},
			err: fmt.Errorf("secrets \"MISSING\" not found"),
		},
		{
			name: "ValidSSHPrivateKey",
			gitAuth: &shared.GitAuthConfig{
				SSHAuth: &shared.SSHAuth{
					SSHPrivateKey: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      sshPrivateKey.Name,
								Key:       "sshPrivateKey",
							},
						},
					},
				},
			},
			expected: &auto.GitAuth{
				SSHPrivateKey: "very secret key",
			},
		},
		{
			name: "ValidSSHPrivateKeyWithPassword",
			gitAuth: &shared.GitAuthConfig{
				SSHAuth: &shared.SSHAuth{
					SSHPrivateKey: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      sshPrivateKeyWithPassword.Name,
								Key:       "sshPrivateKey",
							},
						},
					},
					Password: &shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      sshPrivateKeyWithPassword.Name,
								Key:       "password",
							},
						},
					},
				},
			},
			expected: &auto.GitAuth{
				SSHPrivateKey: "very secret key",
				Password:      "moar secret password",
			},
		},
		{
			name: "ValidAccessToken",
			gitAuth: &shared.GitAuthConfig{
				PersonalAccessToken: &shared.ResourceRef{
					SelectorType: "Secret",
					ResourceSelector: shared.ResourceSelector{
						SecretRef: &shared.SecretSelector{
							Namespace: namespace,
							Name:      accessToken.Name,
							Key:       "accessToken",
						},
					},
				},
			},
			expected: &auto.GitAuth{
				PersonalAccessToken: "super secret access token",
			},
		},
		{
			name: "ValidBasicAuth",
			gitAuth: &shared.GitAuthConfig{
				BasicAuth: &shared.BasicAuth{
					UserName: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      basicAuth.Name,
								Key:       "username",
							},
						},
					},
					Password: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      basicAuth.Name,
								Key:       "password",
							},
						},
					},
				},
			},
			expected: &auto.GitAuth{
				Username: "not so secret username",
				Password: "very secret password",
			},
		},
		{
			name: "BasicAuthWithoutPassword",
			gitAuth: &shared.GitAuthConfig{
				BasicAuth: &shared.BasicAuth{
					UserName: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      basicAuthWithoutPassword.Name,
								Key:       "username",
							},
						},
					},
					Password: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      basicAuthWithoutPassword.Name,
								Key:       "password",
							},
						},
					},
				},
			},
			err: errors.New("No key \"password\" found in secret test/basicAuthWithoutPassword"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := newStackReconcilerSession(log, shared.StackSpec{
				GitSource: &shared.GitSource{GitAuth: test.gitAuth},
			}, client, scheme.Scheme, namespace)
			gitAuth, err := session.SetupGitAuth(context.TODO())
			if test.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.err.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expected, gitAuth)
		})
	}
}

func TestSetupWorkspace(t *testing.T) {
	scheme.Scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "pulumi.com", Version: "v1", Kind: "Stack"},
		&v1.Stack{},
	)
	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		Build()

	for _, test := range []struct {
		name      string
		workspace *autov1alpha1.EmbeddedWorkspaceTemplateSpec
		expected  *autov1alpha1.Workspace
		err       error
	}{
		{
			name: "MergeWorkspaceSpec",
			workspace: &autov1alpha1.EmbeddedWorkspaceTemplateSpec{
				Metadata: autov1alpha1.EmbeddedObjectMeta{
					Labels: map[string]string{
						"custom": "label",
					},
				},
				Spec: &autov1alpha1.WorkspaceSpec{
					Image:              "custom-image",
					ServiceAccountName: "custom-service-account",
					PodTemplate: &autov1alpha1.EmbeddedPodTemplateSpec{
						Metadata: autov1alpha1.EmbeddedObjectMeta{
							Annotations: map[string]string{
								"custom": "pod-annotation",
							},
						},
					},
				},
			},
			expected: &autov1alpha1.Workspace{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Workspace",
					APIVersion: "auto.pulumi.com/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Labels: map[string]string{
						"app.kubernetes.io/component":  "stack",
						"app.kubernetes.io/instance":   "",
						"app.kubernetes.io/managed-by": "pulumi-kubernetes-operator",
						"app.kubernetes.io/name":       "pulumi",
						"custom":                       "label",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "pulumi.com/v1", Kind: "Stack",
							BlockOwnerDeletion: ptr.To(true), Controller: ptr.To(true),
						},
					},
				},
				Spec: autov1alpha1.WorkspaceSpec{
					PodTemplate: &autov1alpha1.EmbeddedPodTemplateSpec{
						Metadata: autov1alpha1.EmbeddedObjectMeta{
							Annotations: map[string]string{
								"custom": "pod-annotation",
							},
						},
						Spec: &corev1.PodSpec{},
					},
					ServiceAccountName: "custom-service-account",
					Image:              "custom-image",
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			log := testr.New(t).WithValues("Request.Test", t.Name())
			session := newStackReconcilerSession(log, shared.StackSpec{
				WorkspaceTemplate: test.workspace,
			}, client, scheme.Scheme, namespace)
			require.NoError(t, session.NewWorkspace(&v1.Stack{Spec: session.stack}))

			err := session.setupWorkspace(context.Background())
			if test.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.err.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expected, session.ws)
		})
	}
}

func (suite *GitAuthTestSuite) TestSetupGitAuthWithRefs() {
	t := suite.T()
	t.Skip() // https://github.com/pulumi/pulumi-kubernetes-operator/pull/658
	log := testr.New(t).WithValues("Request.Test", "TestSetupGitAuthWithSecrets")

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"SECRET1": []byte("very secret"),
			"SECRET2": []byte("moar secret"),
		},
		Type: "Opaque",
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(secret).
		Build()

	for _, test := range []struct {
		name     string
		gitAuth  *shared.GitAuthConfig
		expected *auto.GitAuth
		err      error
	}{
		{
			name:     "NilGitAuth",
			expected: &auto.GitAuth{},
		},
		{
			name:    "EmptyGitAuth",
			gitAuth: &shared.GitAuthConfig{},
			err:     fmt.Errorf("gitAuth config must specify exactly one of 'personalAccessToken', 'sshPrivateKey' or 'basicAuth'"),
		},
		{
			name: "GitAuthValidSecretReference",
			gitAuth: &shared.GitAuthConfig{
				PersonalAccessToken: &shared.ResourceRef{
					SelectorType: shared.ResourceSelectorSecret,
					ResourceSelector: shared.ResourceSelector{
						SecretRef: &shared.SecretSelector{
							Namespace: namespace,
							Name:      secret.Name,
							Key:       "SECRET1",
						},
					},
				},
			},
			expected: &auto.GitAuth{
				PersonalAccessToken: "very secret",
			},
		},
		{
			name: "GitAuthValidFileReference",
			gitAuth: &shared.GitAuthConfig{
				PersonalAccessToken: &shared.ResourceRef{
					SelectorType: shared.ResourceSelectorFS,
					ResourceSelector: shared.ResourceSelector{
						FileSystem: &shared.FSSelector{
							Path: suite.f,
						},
					},
				},
			},
			expected: &auto.GitAuth{
				PersonalAccessToken: "super secret",
			},
		},
		{
			name: "GitAuthInvalidFileReference",
			gitAuth: &shared.GitAuthConfig{
				PersonalAccessToken: &shared.ResourceRef{
					SelectorType: shared.ResourceSelectorFS,
					ResourceSelector: shared.ResourceSelector{
						FileSystem: &shared.FSSelector{
							Path: "/tmp/!@#@!#",
						},
					},
				},
			},
			err: fmt.Errorf("open /tmp/!@#@!#: no such file or directory"),
		},
		{
			name: "GitAuthValidEnvVarReference",
			gitAuth: &shared.GitAuthConfig{
				PersonalAccessToken: &shared.ResourceRef{
					SelectorType: shared.ResourceSelectorEnv,
					ResourceSelector: shared.ResourceSelector{
						Env: &shared.EnvSelector{
							Name: "SECRET3",
						},
					},
				},
			},
			expected: &auto.GitAuth{
				PersonalAccessToken: "so secret",
			},
		},
		{
			name: "GitAuthInvalidEnvReference",
			gitAuth: &shared.GitAuthConfig{
				PersonalAccessToken: &shared.ResourceRef{
					SelectorType: shared.ResourceSelectorEnv,
					ResourceSelector: shared.ResourceSelector{
						Env: &shared.EnvSelector{
							Name: "MISSING",
						},
					},
				},
			},
			err: fmt.Errorf("missing value for environment variable: MISSING"),
		},
		{
			name: "GitAuthValidSSHAuthWithoutPassword",
			gitAuth: &shared.GitAuthConfig{
				SSHAuth: &shared.SSHAuth{
					SSHPrivateKey: shared.ResourceRef{
						SelectorType: shared.ResourceSelectorSecret,
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      secret.Name,
								Key:       "SECRET1",
							},
						},
					},
				},
			},
			expected: &auto.GitAuth{
				SSHPrivateKey: "very secret",
			},
		},
		{
			name: "GitAuthValidSSHAuthWithPassword",
			gitAuth: &shared.GitAuthConfig{
				SSHAuth: &shared.SSHAuth{
					SSHPrivateKey: shared.ResourceRef{
						SelectorType: shared.ResourceSelectorSecret,
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      secret.Name,
								Key:       "SECRET1",
							},
						},
					},
					Password: &shared.ResourceRef{
						SelectorType: shared.ResourceSelectorSecret,
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      secret.Name,
								Key:       "SECRET2",
							},
						},
					},
				},
			},
			expected: &auto.GitAuth{
				SSHPrivateKey: "very secret",
				Password:      "moar secret",
			},
		},
		{
			name: "GitAuthInvalidSSHAuthWithPassword",
			gitAuth: &shared.GitAuthConfig{
				SSHAuth: &shared.SSHAuth{
					SSHPrivateKey: shared.ResourceRef{
						SelectorType: shared.ResourceSelectorSecret,
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      secret.Name,
								Key:       "SECRET1",
							},
						},
					},
					Password: &shared.ResourceRef{
						SelectorType: shared.ResourceSelectorSecret,
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      secret.Name,
								Key:       "MISSING",
							},
						},
					},
				},
			},
			err: fmt.Errorf("resolving gitAuth SSH password: No key \"MISSING\" found in secret test/fake-secret"),
		},
		{
			name: "GitAuthValidBasicAuth",
			gitAuth: &shared.GitAuthConfig{
				BasicAuth: &shared.BasicAuth{
					UserName: shared.ResourceRef{
						SelectorType: shared.ResourceSelectorSecret,
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      secret.Name,
								Key:       "SECRET1",
							},
						},
					},
					Password: shared.ResourceRef{
						SelectorType: shared.ResourceSelectorSecret,
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      secret.Name,
								Key:       "SECRET2",
							},
						},
					},
				},
			},
			expected: &auto.GitAuth{
				Username: "very secret",
				Password: "moar secret",
			},
		},
		{
			name: "GitAuthInvalidSecretReference",
			gitAuth: &shared.GitAuthConfig{
				PersonalAccessToken: &shared.ResourceRef{
					SelectorType: shared.ResourceSelectorSecret,
					ResourceSelector: shared.ResourceSelector{
						SecretRef: &shared.SecretSelector{
							Namespace: namespace,
							Name:      secret.Name,
							Key:       "MISSING",
						},
					},
				},
			},
			err: fmt.Errorf("resolving gitAuth personal access token: No key \"MISSING\" found in secret test/fake-secret"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := newStackReconcilerSession(log, shared.StackSpec{
				GitSource: &shared.GitSource{
					GitAuth: test.gitAuth,
				},
			}, client, scheme.Scheme, namespace)
			gitAuth, err := session.SetupGitAuth(context.TODO())
			if test.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.err.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expected, gitAuth)
		})
	}
}
