// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulumi

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/pulumi/pulumi-kubernetes-operator/operator/api/auto/v1alpha1"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/shared"
	v1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/v1"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

var (
	_sshPrivateKey = &corev1.Secret{
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
	_sshPrivateKeyWithPassword = &corev1.Secret{
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
	_accessToken = &corev1.Secret{
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
	_basicAuth = &corev1.Secret{
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
	_basicAuthWithoutPassword = &corev1.Secret{
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
)

func TestSetupGitAuthWithSecrets(t *testing.T) {
	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(_sshPrivateKey, _sshPrivateKeyWithPassword, _accessToken, _basicAuth, _basicAuthWithoutPassword).
		Build()

	for _, test := range []struct {
		name          string
		gitAuth       *shared.GitAuthConfig
		gitAuthSecret string
		expected      *auto.GitAuth
		err           error
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
			name:          "InvalidSecretName (gitAuthSecret)",
			gitAuthSecret: "MISSING",
			err:           fmt.Errorf("secrets \"MISSING\" not found"),
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
								Name:      _sshPrivateKey.Name,
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
								Name:      _sshPrivateKeyWithPassword.Name,
								Key:       "sshPrivateKey",
							},
						},
					},
					Password: &shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      _sshPrivateKeyWithPassword.Name,
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
			name:          "ValidSSHPrivateKeyWithPassword (gitAuthSecret)",
			gitAuthSecret: _sshPrivateKeyWithPassword.Name,
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
							Name:      _accessToken.Name,
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
			name:          "ValidAccessToken (gitAuthSecret)",
			gitAuthSecret: _accessToken.Name,
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
								Name:      _basicAuth.Name,
								Key:       "username",
							},
						},
					},
					Password: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      _basicAuth.Name,
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
			name:          "ValidBasicAuth (gitAuthSecret)",
			gitAuthSecret: _basicAuth.Name,
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
								Name:      _basicAuthWithoutPassword.Name,
								Key:       "username",
							},
						},
					},
					Password: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      _basicAuthWithoutPassword.Name,
								Key:       "password",
							},
						},
					},
				},
			},
			err: errors.New("no key \"password\" found in secret test/basicAuthWithoutPassword"),
		},
		{
			name:          "BasicAuthWithoutPassword (gitAuthSecret)",
			gitAuthSecret: _basicAuthWithoutPassword.Name,
			err:           errors.New(`no key "password" found in secret test/basicAuthWithoutPassword`),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			log := testr.New(t).WithValues("Request.Test", t.Name())
			session := newStackReconcilerSession(log, shared.StackSpec{
				GitSource: &shared.GitSource{
					GitAuth:       test.gitAuth,
					GitAuthSecret: test.gitAuthSecret,
				},
			}, client, scheme.Scheme, namespace)
			gitAuth, err := session.resolveGitAuth(context.TODO())
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
		name     string
		stack    shared.StackSpec
		expected *autov1alpha1.Workspace
		err      error
	}{
		{
			name: "MergeWorkspaceSpec",
			stack: shared.StackSpec{
				SecretRefs: map[string]shared.ResourceRef{
					"boo": {
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Name: "secret-name",
							},
						},
					},
				},
				WorkspaceTemplate: &autov1alpha1.EmbeddedWorkspaceTemplateSpec{
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
							Spec: &corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:         "pulumi",
										VolumeMounts: []corev1.VolumeMount{{Name: "foo", MountPath: "/foo"}},
									},
								},
								Volumes: []corev1.Volume{
									{Name: "foo"},
								},
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
						Spec: &corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "pulumi",
									VolumeMounts: []corev1.VolumeMount{
										{Name: "foo", MountPath: "/foo"},
										{Name: "secret-secret-name", MountPath: "/var/run/secrets/stacks.pulumi.com/secrets/secret-name"},
									},
								},
							},
							Volumes: []corev1.Volume{
								{Name: "foo"},
								{Name: "secret-secret-name", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "secret-name"}}},
							},
						},
					},
					ServiceAccountName: "custom-service-account",
					Image:              "custom-image",
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			log := testr.New(t).WithValues("Request.Test", t.Name())
			session := newStackReconcilerSession(log, test.stack, client, scheme.Scheme, namespace)
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

func TestSetupWorkspaceFromGitSource(t *testing.T) {
	scheme.Scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "pulumi.com", Version: "v1", Kind: "Stack"},
		&v1.Stack{},
	)
	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(_sshPrivateKey, _sshPrivateKeyWithPassword, _accessToken, _basicAuth, _basicAuthWithoutPassword).
		Build()

	for _, test := range []struct {
		name          string
		gitAuth       *shared.GitAuthConfig
		gitAuthSecret string
		expected      *v1alpha1.WorkspaceSpec
		err           error
	}{
		{
			name: "SSHPrivateKeyWithPassword",
			gitAuth: &shared.GitAuthConfig{
				SSHAuth: &shared.SSHAuth{
					SSHPrivateKey: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      _sshPrivateKeyWithPassword.Name,
								Key:       "sshPrivateKey",
							},
						},
					},
					Password: &shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      _sshPrivateKeyWithPassword.Name,
								Key:       "password",
							},
						},
					},
				},
			},
			expected: &v1alpha1.WorkspaceSpec{
				PodTemplate: &v1alpha1.EmbeddedPodTemplateSpec{
					Spec: &corev1.PodSpec{
						Containers: []corev1.Container{{Name: "pulumi"}},
					},
				},
				Git: &v1alpha1.GitSource{
					Ref: "commit-hash",
					Auth: &v1alpha1.GitAuth{
						SSHPrivateKey: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: _sshPrivateKeyWithPassword.Name,
							},
							Key: "sshPrivateKey",
						},
						Password: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: _sshPrivateKeyWithPassword.Name,
							},
							Key: "password",
						},
					},
				},
			},
		},
		{
			name: "AccessToken",
			gitAuth: &shared.GitAuthConfig{
				PersonalAccessToken: &shared.ResourceRef{
					SelectorType: "Secret",
					ResourceSelector: shared.ResourceSelector{
						SecretRef: &shared.SecretSelector{
							Namespace: namespace,
							Name:      _accessToken.Name,
							Key:       "accessToken",
						},
					},
				},
			},
			expected: &v1alpha1.WorkspaceSpec{
				PodTemplate: &v1alpha1.EmbeddedPodTemplateSpec{
					Spec: &corev1.PodSpec{
						Containers: []corev1.Container{{Name: "pulumi"}},
					},
				},
				Git: &v1alpha1.GitSource{
					Ref: "commit-hash",
					Auth: &v1alpha1.GitAuth{
						Token: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: _accessToken.Name,
							},
							Key: "accessToken",
						},
					},
				},
			},
		},
		{
			name: "BasicAuth",
			gitAuth: &shared.GitAuthConfig{
				BasicAuth: &shared.BasicAuth{
					UserName: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      _basicAuth.Name,
								Key:       "username",
							},
						},
					},
					Password: shared.ResourceRef{
						SelectorType: "Secret",
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Namespace: namespace,
								Name:      _basicAuth.Name,
								Key:       "password",
							},
						},
					},
				},
			},
			expected: &v1alpha1.WorkspaceSpec{
				PodTemplate: &v1alpha1.EmbeddedPodTemplateSpec{
					Spec: &corev1.PodSpec{
						Containers: []corev1.Container{{Name: "pulumi"}},
					},
				},
				Git: &v1alpha1.GitSource{
					Ref: "commit-hash",
					Auth: &v1alpha1.GitAuth{
						Password: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: _basicAuth.Name,
							},
							Key: "password",
						},
						Username: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: _basicAuth.Name,
							},
							Key: "username",
						},
					},
				},
			},
		},
		{
			name:          "GitAuthSecret",
			gitAuthSecret: _accessToken.Name,
			expected: &v1alpha1.WorkspaceSpec{
				PodTemplate: &v1alpha1.EmbeddedPodTemplateSpec{
					Spec: &corev1.PodSpec{
						Containers: []corev1.Container{{Name: "pulumi"}},
					},
				},
				Git: &v1alpha1.GitSource{
					Ref: "commit-hash",
					Auth: &v1alpha1.GitAuth{
						SSHPrivateKey: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: _accessToken.Name,
							},
							Key:      "sshPrivateKey",
							Optional: ptr.To(true),
						},
						Password: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: _accessToken.Name,
							},
							Key:      "password",
							Optional: ptr.To(true),
						},
						Username: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: _accessToken.Name,
							},
							Key:      "username",
							Optional: ptr.To(true),
						},
						Token: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: _accessToken.Name,
							},
							Key:      "accessToken",
							Optional: ptr.To(true),
						},
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			log := testr.New(t).WithValues("Request.Test", t.Name())
			session := newStackReconcilerSession(log, shared.StackSpec{
				GitSource: &shared.GitSource{
					GitAuth:       test.gitAuth,
					GitAuthSecret: test.gitAuthSecret,
				},
			}, client, scheme.Scheme, namespace)
			require.NoError(t, session.NewWorkspace(&v1.Stack{
				Spec: session.stack,
			}))

			err := session.setupWorkspaceFromGitSource(context.TODO(), "commit-hash")
			if test.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.err.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expected, &session.ws.Spec)
		})
	}
}

func TestSetupGitAuthWithRefs(t *testing.T) {
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
			err: fmt.Errorf("FS selectors are no longer supported in v2, please use a secret reference instead"),
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
			err: fmt.Errorf("Env selectors are no longer supported in v2, please use a secret reference instead"),
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
			err: fmt.Errorf("Env selectors are no longer supported in v2, please use a secret reference instead"),
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
			err: fmt.Errorf("resolving gitAuth SSH password: no key \"MISSING\" found in secret test/fake-secret"),
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
			err: fmt.Errorf("resolving gitAuth personal access token: no key \"MISSING\" found in secret test/fake-secret"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := newStackReconcilerSession(log, shared.StackSpec{
				GitSource: &shared.GitSource{
					GitAuth: test.gitAuth,
				},
			}, client, scheme.Scheme, namespace)
			gitAuth, err := session.resolveGitAuth(context.TODO())
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
