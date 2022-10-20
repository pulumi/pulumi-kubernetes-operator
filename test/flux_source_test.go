// Copyright 2022, Pulumi Corporation.  All rights reserved.

package tests

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

func makeFixtureIntoArtifact(fixture string) (_checksum string, _tarball []byte) {
	buf := &bytes.Buffer{}
	hash := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(buf, hash))
	tarWriter := tar.NewWriter(gz)

	walkErr := filepath.Walk(fixture, func(fullpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fullpath == fixture {
			return nil
		}
		path := fullpath[len(fixture)+1:] // get the bit after "testdata/fixture"
		if info.IsDir() {
			// Do I need a header?
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, "") // assumes no symlinks!
		if err != nil {
			return err
		}
		hdr.Name = path
		if err = tarWriter.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(fullpath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tarWriter, f)
		return err
	})
	ExpectWithOffset(1, walkErr).NotTo(HaveOccurred())

	ExpectWithOffset(1, tarWriter.Close()).To(Succeed())
	ExpectWithOffset(1, gz.Close()).To(Succeed()) // make sure it flushes
	return fmt.Sprintf("%x", hash.Sum(nil)), buf.Bytes()
}

var _ = Describe("Flux source integration", func() {

	var (
		backendDir string
		createCRD  sync.Once
		kubeconfig string
	)

	_ = func(args ...string) {
		kubectl := exec.Command("kubectl", args...)
		kubectl.Env = append(kubectl.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
		kubectl.Stdout = os.Stdout
		kubectl.Stderr = os.Stderr
		kubectl.Run()
	}

	BeforeEach(func() {
		var err error
		backendDir, err = os.MkdirTemp("", "pulumi-test")
		Expect(err).NotTo(HaveOccurred())
		// we need this from time to time for troubleshooting
		kubeconfig = writeKubeconfig(backendDir)

		// This is really a global initialisation, but I want to do it here rather than in
		// BeforeSuite so it's near where it's used.
		createCRD.Do(func() {
			// tell the client about CRDs
			apiextensionsv1.AddToScheme(k8sClient.Scheme())

			// install our minimal CRD
			bs, err := os.ReadFile("testdata/fluxsource_crd.yaml")
			Expect(err).NotTo(HaveOccurred())
			var crd apiextensionsv1.CustomResourceDefinition
			Expect(yaml.Unmarshal(bs, &crd)).To(Succeed())
			Expect(k8sClient.Create(context.TODO(), &crd)).To(Succeed())
			// register with the client
			gvk := schema.FromAPIVersionAndKind("source.pulumi.com/v1", "Fake")
			k8sClient.Scheme().AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
			gvk.Kind += "List"
			k8sClient.Scheme().AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})
			// I don't know why this is so, but one seems to have to fail at least once before the
			// client is happy to deal with the new kind.
			s := &unstructured.Unstructured{}
			s.SetKind("Fake")
			s.SetAPIVersion("source.pulumi.com/v1")
			s.SetName("blank")
			s.SetNamespace("default")
			Eventually(func() bool {
				err := k8sClient.Create(context.TODO(), s)
				if err != nil {
					fmt.Fprintln(GinkgoWriter, "[client error]", err.Error())
				}
				return err == nil
			}, "20s", "1s").Should(BeTrue())
		})
	})

	AfterEach(func() {
		if strings.HasPrefix(backendDir, os.TempDir()) {
			os.RemoveAll(backendDir)
		}
	})

	When("a Stack refers to a missing Flux source", func() {
		var stack *pulumiv1.Stack

		BeforeEach(func() {
			stack = &pulumiv1.Stack{
				Spec: shared.StackSpec{
					Stack:   randString(),
					Backend: fmt.Sprintf("file://%s", backendDir),
					EnvRefs: map[string]shared.ResourceRef{
						"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
					},
					FluxSource: &shared.FluxSource{
						SourceRef: shared.FluxSourceReference{
							APIVersion: "source.pulumi.com/v1",
							Kind:       "Fake",
							Name:       "does-not-exist",
						},
					},
				},
			}
			stack.Name = "missing-source"
			stack.Namespace = "default"
		})

		JustBeforeEach(func() {
			stack.Name += ("-" + randString())
			Expect(k8sClient.Create(context.TODO(), stack)).To(Succeed())
		})

		AfterEach(func() {
			deleteAndWaitForFinalization(stack)
		})

		It("is marked as failed and stalled", func() {
			waitForStackFailure(stack)
			Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.StalledCondition)).To(BeTrue())
			Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReadyCondition)).To(BeFalse())
			Expect(apimeta.FindStatusCondition(stack.Status.Conditions, pulumiv1.ReconcilingCondition)).To(BeNil())
		})

		When("the source is an unknown group/kind", func() {
			BeforeEach(func() {
				stack.Name = "unknown-source-kind"
				stack.Spec.FluxSource.SourceRef.APIVersion = "doesnotexist/v1"
			})

			It("is marked as failed and to be retried", func() {
				waitForStackFailure(stack)
				// When this is present it could say that it's retrying, or that it's in progress; since
				// it's run through at least once for us to see a failed state above, either indicates a
				// retry.
				Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReconcilingCondition)).To(BeTrue())
				Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReadyCondition)).To(BeFalse())
				Expect(apimeta.FindStatusCondition(stack.Status.Conditions, pulumiv1.StalledCondition)).To(BeNil())
			})
		})
	})

	When("a Stack refers to a Flux source with a latest artifact", func() {
		var (
			artifactServer   *httptest.Server
			artifactURL      string
			artifactChecksum string
			artifactRevision string
			source           *unstructured.Unstructured
			stack            *pulumiv1.Stack
		)

		BeforeEach(func() {
			checksum, tarballBytes := makeFixtureIntoArtifact("testdata/success")
			path := "/" + randString()
			mux := http.NewServeMux()
			mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Body.Close()
				w.WriteHeader(200)
				w.Write(tarballBytes)
			}))
			artifactServer = httptest.NewServer(mux)
			artifactURL = artifactServer.URL + path
			artifactRevision = randString()
			artifactChecksum = checksum

			source = &unstructured.Unstructured{}
			sourceStatus := map[string]interface{}{
				"artifact": map[string]interface{}{
					"path":     "irrelevant",
					"url":      artifactURL,
					"revision": artifactRevision,
					"checksum": artifactChecksum,
				},
			}
			source.SetKind("Fake")
			source.SetAPIVersion("source.pulumi.com/v1")
			source.SetName(randString())
			source.SetNamespace("default")
			Expect(k8sClient.Create(context.TODO(), source)).To(Succeed())
			unstructured.SetNestedMap(source.Object, sourceStatus, "status")
			Expect(k8sClient.Status().Update(context.TODO(), source)).To(Succeed())

			stack = &pulumiv1.Stack{
				Spec: shared.StackSpec{
					Stack:   randString(),
					Backend: fmt.Sprintf("file://%s", backendDir),
					EnvRefs: map[string]shared.ResourceRef{
						"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
						"KUBECONFIG":               shared.NewLiteralResourceRef(kubeconfig),
					},
					FluxSource: &shared.FluxSource{
						SourceRef: shared.FluxSourceReference{
							APIVersion: "source.pulumi.com/v1",
							Kind:       "Fake",
							Name:       source.GetName(),
						},
					},
				},
			}
			stack.Name = "flux-source"
			stack.Namespace = "default"
		})

		// This is done just before testing the outcome, so that any adjustments to the source, or
		// the stack, can be made before the stack is processed.
		JustBeforeEach(func() {
			stack.Name += ("-" + randString())
			Expect(k8sClient.Create(context.TODO(), stack)).To(Succeed())
		})

		AfterEach(func() {
			deleteAndWaitForFinalization(stack)
			deleteAndWaitForFinalization(source)
			artifactServer.Close()
		})

		When("the stack runs to success", func() {
			JustBeforeEach(func() {
				waitForStackSuccess(stack)
			})

			It("records the revision from the source", func() {
				Expect(stack.Status.LastUpdate).NotTo(BeNil())
				Expect(stack.Status.LastUpdate.LastSuccessfulCommit).To(Equal(artifactRevision))
			})
		})

		When("the source is updated after a run", func() {
			var newArtifactRevision string

			JustBeforeEach(func() {
				// wait for one go around
				waitForStackSuccess(stack)

				newArtifactRevision = randString()
				sourceStatus := map[string]interface{}{
					"artifact": map[string]interface{}{
						"path":     "irrelevant",
						"url":      artifactURL,
						"revision": newArtifactRevision,
						"checksum": artifactChecksum,
					},
				}
				unstructured.SetNestedMap(source.Object, sourceStatus, "status")
				resetWaitForStack()
				Expect(k8sClient.Status().Update(context.TODO(), source)).To(Succeed())
			})

			It("runs the stack again at the new revision", func() {
				waitForStackSuccess(stack)
				Expect(stack.Status.LastUpdate.LastSuccessfulCommit).To(Equal(newArtifactRevision))
			})
		})

		When("the source object is explicitly marked as not ready", func() {
			BeforeEach(func() {
				notready := map[string]interface{}{
					"type":               "Ready", // ) the type "Ready" and Status != "True" are enough to
					"status":             "False", // ) mark it as unready.
					"reason":             "ReasonIrrelevant",
					"message":            "the message is also irrelevant",
					"lastTransitionTime": "2022-10-10T14:18:22Z",
				}
				conditions := []interface{}{notready}
				unstructured.SetNestedSlice(source.Object, conditions, "status", "conditions")
				Expect(k8sClient.Status().Update(context.TODO(), source)).To(Succeed())
				stack.Name = "source-not-ready"
			})

			It("marks the stack as failed and to be retried", func() {
				waitForStackFailure(stack)
				Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReconcilingCondition)).To(BeTrue())
				Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReadyCondition)).To(BeFalse())

				By("marking the source as ready, the stack can run")
				ready := map[string]interface{}{
					"type":               "Ready",
					"status":             "True",
					"reason":             "ReasonIrrelevant",
					"message":            "the message is also irrelevant",
					"lastTransitionTime": "2022-10-10T14:58:22Z",
				}
				conditions := []interface{}{ready}
				unstructured.SetNestedSlice(source.Object, conditions, "status", "conditions")
				Expect(k8sClient.Status().Update(context.TODO(), source)).To(Succeed())

				waitForStackSuccess(stack)
				Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReadyCondition)).To(BeTrue())
			})
		})

		When("the checksum is wrong", func() {
			BeforeEach(func() {
				unstructured.SetNestedField(source.Object, "not-the-right-checksum",
					"status", "artifact", "checksum")
				Expect(k8sClient.Status().Update(context.TODO(), source)).To(Succeed())
				stack.Name = "source-bad-checksum"
			})

			It("rejects the tarball and fails with a retry", func() {
				resetWaitForStack()
				waitForStackFailure(stack)
				Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReconcilingCondition)).To(BeTrue())
				Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReadyCondition)).To(BeFalse())
			})
		})

		When("the artifact cannot be fetched", func() {
			BeforeEach(func() {
				unstructured.SetNestedField(source.Object, artifactServer.URL+"/bogus/path/to/artifact.tar.gz",
					"status", "artifact", "url")
				Expect(k8sClient.Status().Update(context.TODO(), source)).To(Succeed())
				stack.Name = "source-unavailable"
			})

			It("marks the Stack as failed and to be retried", func() {
				waitForStackFailure(stack)
				Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReconcilingCondition)).To(BeTrue())
				Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReadyCondition)).To(BeFalse())
			})
		})
	})
})
