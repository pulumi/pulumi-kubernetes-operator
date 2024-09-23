/*
Copyright 2024.

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

package pulumi

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/opencontainers/go-digest"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/yaml"
)

const (
	ProgramControllerName = "program-controller"
	pulumiProjectFileName = "Pulumi.yaml"
)

// ProgramReconciler reconciles a Program object
type ProgramReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	ProgramHandler *ProgramHandler
}

// ProjectFile contains additional 'project' metadata fields required for Pulumi to run on a ProgramSpec.
type ProjectFile struct {
	Name    string `json:"name"`
	Runtime string `json:"runtime"`
	pulumiv1.ProgramSpec

	timeModified metav1.Time // this field is not to be serialized
}

// tarProgram creates a tarball of the Pulumi project file and writes it to the provided writer.
func tarProgram(data []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	// Create tar header for the Pulumi.yaml file.
	hdr := &tar.Header{
		Name:     pulumiProjectFileName,
		Size:     int64(len(data)),
		Mode:     509,
		Typeflag: tar.TypeReg,
	}

	err := tw.WriteHeader(hdr)
	if err != nil {
		return nil, err
	}

	num, err := tw.Write(data)
	if err != nil {
		return nil, err
	}

	// Ensure that the number of bytes tarred is identical to the original data.
	if num != len(data) {
		return nil, errors.New("tarred data size mismatch")
	}

	return buf.Bytes(), nil
}

// gzipFile compresses the input data using gzip.
func gzipFile(input []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	num, err := gw.Write(input)
	if err != nil {
		return nil, fmt.Errorf("unable to write to gzip writer: %w", err)
	}

	if num != len(input) {
		return nil, fmt.Errorf("gzip writer wrote %d bytes, expected %d", num, len(input))
	}

	err = gw.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// urlPathToProgramIdentifier converts a URL path to a Program identifier.
func urlPathToProgramIdentifier(urlPath string) (namespace string, name string, err error) {
	pathComp := strings.Split(urlPath, "/")
	if len(pathComp) != 2 {
		return "", "", errors.New("invalid program identifier, unable to locate Program")
	}

	return pathComp[0], pathComp[1], nil
}

type ProgramHandler struct {
	// address is the advertised address of the HTTP server serving the Program objects.
	address string
	// k8sclient is the Kubernetes client.
	k8sClient client.Client
}

func NewProgramHandler(k8sClient client.Client, address string) *ProgramHandler {
	return &ProgramHandler{
		address:   address,
		k8sClient: k8sClient,
	}
}

func (p *ProgramHandler) Address() string {
	return p.address
}

// CreateProgramURL creates a URL path for a Program.
func (p *ProgramHandler) CreateProgramURL(namespace, name string) string {
	address := p.address

	if !(strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://")) {
		address = "http://" + address
	}

	path, _ := url.JoinPath(address, "programs", namespace, name)
	return path
}

// HandleProgramServing is a HTTP handler for serving a Program as a valid Pulumi project.
// For simplicity, we do not store any of these files on disk, but rather, rely on generating them on the fly.
func (p *ProgramHandler) HandleProgramServing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract the hashed URL path to identify the program.
		programIdentifier := r.URL.Path[len("/programs/"):]
		if programIdentifier == "" {
			http.Error(w, "Program identifier is required.", http.StatusBadRequest)
			return
		}

		namespace, name, err := urlPathToProgramIdentifier(programIdentifier)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		proj, err := getProjectFile(context.Background(), p.k8sClient, namespace, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Marshal the project file to YAML.
		data, err := yaml.Marshal(proj)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create a tarball of the project file and write it to the response.
		out, err := tarProgram(data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Compress the tarball using gzip.
		out, err = gzipFile(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", "attachment; filename=project.tar.gz")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(out)))
		w.Write([]byte(out))
	}
}

// getProjectFile retrieves a Program object from the Kubernetes API server and returns a valid Pulumi Yaml project file from it.
func getProjectFile(ctx context.Context, kubeClient client.Client, namespace, name string) (*ProjectFile, error) {
	program := new(pulumiv1.Program)
	programKey := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}

	err := kubeClient.Get(ctx, programKey, program)
	if err != nil {
		return nil, err
	}

	project := programToProject(program)

	return project, nil
}

// programToProject wraps a Program object to a ProjectFile.
func programToProject(program *pulumiv1.Program) *ProjectFile {
	return &ProjectFile{
		Name:        program.Name,
		Runtime:     "yaml",
		ProgramSpec: program.Program,

		timeModified: program.Status.LastResyncTime,
	}
}

//+kubebuilder:rbac:groups=pulumi.com,resources=programs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pulumi.com,resources=programs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pulumi.com,resources=programs/finalizers,verbs=update

// Reconcile reconciles the Program object.
func (r *ProgramReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling Program")

	// Fetch the Program object.
	program := new(pulumiv1.Program)
	if err := r.Get(ctx, req.NamespacedName, program); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Update the status of the Program object to contain an updated URL.
	program.Status.Artifact = &pulumiv1.Artifact{
		URL: r.ProgramHandler.CreateProgramURL(program.Namespace, program.Name),
	}
	program.Status.LastResyncTime = metav1.Now()
	program.Status.ObservedGeneration = program.GetGeneration()

	// Calculate and store the sha256 digest of the tarball.
	// This is necessary for the Flux fetcher to verify the integrity of the artifact.
	log.Info("Calculating digest hash for Program artifact")
	project := programToProject(program)
	data, err := yaml.Marshal(project)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to marshal project file to calculate digest hash: %w", err)
	}

	tarData, err := tarProgram(data)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to create tarball of project file: %w", err)
	}
	gzData, err := gzipFile(tarData)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to compress tarball: %w", err)
	}
	program.Status.Artifact.Digest = calculateDigest(gzData)

	// Update the status of the Program object.
	log.Info("Updating Program status")
	if err := r.Status().Update(ctx, program, client.FieldOwner(FieldManager)); err != nil {
		log.Error(err, "unable to update Program status", req.NamespacedName.MarshalLog())
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func calculateDigest(data []byte) string {
	return digest.SHA256.FromBytes(data).String()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProgramReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Track metrics about Program resources.
	stackInformer, err := mgr.GetCache().GetInformer(context.Background(), new(pulumiv1.Program))
	if err != nil {
		return err
	}
	if _, err = stackInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    newProgramCallback,
		DeleteFunc: deleteProgramCallback,
	}); err != nil {
		return err
	}

	// Create a new controller and set the options. We will only reconcile on spec changes to the Program resource (new generation).
	return ctrl.NewControllerManagedBy(mgr).
		Named(ProgramControllerName).
		For(new(pulumiv1.Program)).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
