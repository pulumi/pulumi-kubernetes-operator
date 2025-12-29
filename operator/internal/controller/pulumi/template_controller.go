// Copyright 2016-2025, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	pulumiv1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1alpha1"
)

const (
	TemplateControllerName = "template-controller"
	TemplateFinalizerName  = "finalizer.template.pulumi.com"
	InstanceFinalizerName  = "finalizer.instance.template.pulumi.com"
	GeneratedByLabel       = "pulumi.com/generated-by"
	TemplateNameLabel      = "pulumi.com/template-name"
	TemplateNamespaceLabel = "pulumi.com/template-namespace"
	InstanceNameLabel      = "pulumi.com/instance-name"
	InstanceNamespaceLabel = "pulumi.com/instance-namespace"
	GeneratedByValue       = "pulumi-operator"
	TemplateFieldManager   = "pulumi-template-controller"
)

// TemplateReconciler reconciles a Template object
type TemplateReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Recorder            record.EventRecorder
	RESTConfig          *rest.Config
	ApiExtensionsClient apiextensionsclient.Interface
	DynamicClient       dynamic.Interface

	// mu protects the informers map
	mu sync.RWMutex
	// informers tracks dynamic informers for generated CRDs
	informers map[string]cache.SharedIndexInformer
	// stopChans tracks stop channels for informers
	stopChans map[string]chan struct{}
}

// NewTemplateReconciler creates a new TemplateReconciler
func NewTemplateReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
	restConfig *rest.Config,
) (*TemplateReconciler, error) {
	apiExtClient, err := apiextensionsclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create apiextensions client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &TemplateReconciler{
		Client:              c,
		Scheme:              scheme,
		Recorder:            recorder,
		RESTConfig:          restConfig,
		ApiExtensionsClient: apiExtClient,
		DynamicClient:       dynamicClient,
		informers:           make(map[string]cache.SharedIndexInformer),
		stopChans:           make(map[string]chan struct{}),
	}, nil
}

//+kubebuilder:rbac:groups=pulumi.com,resources=pulumitemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pulumi.com,resources=pulumitemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pulumi.com,resources=pulumitemplates/finalizers,verbs=update
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="*",resources="*",verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles the Template object.
func (r *TemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Track reconcile duration for metrics
	startTime := time.Now()
	reconcileResult := "success"
	defer func() {
		RecordTemplateReconcileDuration(req.Namespace, req.Name, startTime, reconcileResult)
	}()

	// Add reconcile ID for log correlation across related operations
	reconcileID := uuid.New().String()[:8] // Use short ID for readability
	log := log.FromContext(ctx).WithValues(
		"reconcileID", reconcileID,
		"template", req.NamespacedName,
	)
	ctx = logr.NewContext(ctx, log)

	// Fetch the Template object.
	template := new(pulumiv1alpha1.Template)
	if err := r.Get(ctx, req.NamespacedName, template); err != nil {
		if apierrors.IsNotFound(err) {
			// Object was deleted, clean up any informers
			r.cleanupInformer(req.NamespacedName.String())
			reconcileResult = "deleted"
			return ctrl.Result{}, nil
		}
		reconcileResult = "error"
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log = log.WithValues("generation", template.Generation)
	ctx = logr.NewContext(ctx, log)
	log.Info("Reconciling Template")

	// Handle deletion
	if !template.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, template)
	}

	// Sync instance statuses from completed Stacks.
	// This is triggered by the Stack watch when a Stack's Ready condition changes.
	if err := r.syncInstanceStatusesFromStacks(ctx, template); err != nil {
		log.Error(err, "Failed to sync instance statuses from Stacks")
		// Don't return error - continue with reconciliation
	}

	// Clean up instance finalizers for instances whose Stacks have been deleted.
	// This is triggered by the Stack watch when a Stack is deleted after destroy completes.
	if err := r.cleanupInstanceFinalizers(ctx, template); err != nil {
		log.Error(err, "Failed to clean up instance finalizers")
		// Don't return error - continue with reconciliation
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(template, TemplateFinalizerName) {
		controllerutil.AddFinalizer(template, TemplateFinalizerName)
		if err := r.Update(ctx, template); err != nil {
			reconcileResult = "error"
			return ctrl.Result{}, err
		}
		reconcileResult = "requeue"
		return ctrl.Result{Requeue: true}, nil
	}

	// Validate the template
	if err := r.validateTemplate(template); err != nil {
		log.Error(err, "Template validation failed")
		r.setCondition(template, pulumiv1alpha1.TemplateConditionTypeSchemaValid, metav1.ConditionFalse,
			pulumiv1alpha1.TemplateReasonSchemaInvalid, err.Error())
		if requeue, updateErr := r.updateStatusWithConflictHandling(ctx, template); updateErr != nil {
			reconcileResult = "error"
			return ctrl.Result{}, updateErr
		} else if requeue {
			reconcileResult = "conflict"
			return ctrl.Result{Requeue: true}, nil
		}
		r.Recorder.Event(template, "Warning", "ValidationFailed", err.Error())
		reconcileResult = "validation_failed"
		return ctrl.Result{}, nil
	}

	r.setCondition(template, pulumiv1alpha1.TemplateConditionTypeSchemaValid, metav1.ConditionTrue,
		pulumiv1alpha1.TemplateReasonSchemaValid, "Schema is valid")

	// Generate and register the CRD
	crd, err := r.generateCRD(template)
	if err != nil {
		log.Error(err, "Failed to generate CRD")
		r.setCondition(template, pulumiv1alpha1.TemplateConditionTypeCRDReady, metav1.ConditionFalse,
			pulumiv1alpha1.TemplateReasonCRDFailed, err.Error())
		if requeue, updateErr := r.updateStatusWithConflictHandling(ctx, template); updateErr != nil {
			reconcileResult = "error"
			return ctrl.Result{}, updateErr
		} else if requeue {
			reconcileResult = "conflict"
			return ctrl.Result{Requeue: true}, nil
		}
		r.Recorder.Event(template, "Warning", "CRDGenerationFailed", err.Error())
		reconcileResult = "crd_generation_failed"
		return ctrl.Result{}, nil
	}

	// Register/update the CRD
	if err := r.registerCRD(ctx, crd, template); err != nil {
		log.Error(err, "Failed to register CRD")
		r.setCondition(template, pulumiv1alpha1.TemplateConditionTypeCRDReady, metav1.ConditionFalse,
			pulumiv1alpha1.TemplateReasonCRDFailed, err.Error())
		if requeue, updateErr := r.updateStatusWithConflictHandling(ctx, template); updateErr != nil {
			reconcileResult = "error"
			return ctrl.Result{}, updateErr
		} else if requeue {
			reconcileResult = "conflict"
			return ctrl.Result{Requeue: true}, nil
		}
		r.Recorder.Event(template, "Warning", "CRDRegistrationFailed", err.Error())
		reconcileResult = "crd_registration_failed"
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Update status with CRD info
	group, version := parseAPIVersion(template.Spec.CRD.APIVersion)
	plural := getPluralName(template.Spec.CRD)
	template.Status.CRD = &pulumiv1alpha1.GeneratedCRDStatus{
		Name:    crd.Name,
		Group:   group,
		Version: version,
		Kind:    template.Spec.CRD.Kind,
		Plural:  plural,
		Ready:   true,
	}

	r.setCondition(template, pulumiv1alpha1.TemplateConditionTypeCRDReady, metav1.ConditionTrue,
		pulumiv1alpha1.TemplateReasonCRDRegistered, fmt.Sprintf("CRD %s registered successfully", crd.Name))

	// Start/update informer for the generated CRD
	if err := r.ensureInformer(ctx, template); err != nil {
		log.Error(err, "Failed to start informer")
		// Don't fail reconciliation, just log and requeue
		reconcileResult = "informer_failed"
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Count instances
	instanceCount, err := r.countInstances(ctx, template)
	if err != nil {
		log.Error(err, "Failed to count instances")
	} else {
		template.Status.InstanceCount = instanceCount
	}

	// Update final status
	template.Status.ObservedGeneration = template.Generation
	now := metav1.Now()
	template.Status.LastReconciled = &now

	r.setCondition(template, pulumiv1alpha1.TemplateConditionTypeReady, metav1.ConditionTrue,
		pulumiv1alpha1.TemplateReasonReconcileSucceeded, "Template is ready")

	if requeue, updateErr := r.updateStatusWithConflictHandling(ctx, template); updateErr != nil {
		log.Error(updateErr, "Failed to update status")
		reconcileResult = "error"
		return ctrl.Result{}, updateErr
	} else if requeue {
		reconcileResult = "conflict"
		return ctrl.Result{Requeue: true}, nil
	}

	r.Recorder.Event(template, "Normal", "Reconciled", fmt.Sprintf("CRD %s is ready", crd.Name))
	log.Info("Template reconciled successfully", "crd", crd.Name)

	// reconcileResult is already "success" by default
	return ctrl.Result{}, nil
}

// handleDeletion handles the deletion of a Template.
func (r *TemplateReconciler) handleDeletion(ctx context.Context, template *pulumiv1alpha1.Template) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Handling deletion of Template")

	if !controllerutil.ContainsFinalizer(template, TemplateFinalizerName) {
		return ctrl.Result{}, nil
	}

	// Before deleting anything, wait for all associated Stacks to complete their destroy operations.
	// This prevents orphaned cloud resources when the Template is deleted.
	pendingStacks, err := r.listPendingStacks(ctx, template)
	if err != nil {
		log.Error(err, "Failed to list pending stacks")
		return ctrl.Result{}, err
	}

	if len(pendingStacks) > 0 {
		log.Info("Waiting for Stacks to complete destroy operations before deleting Template",
			"pendingStacks", len(pendingStacks),
			"stacks", pendingStacks)
		// Requeue to check again - the Stack controller will handle the destroy
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// All Stacks are gone, now we can safely delete Programs
	if err := r.deleteAssociatedPrograms(ctx, template); err != nil {
		log.Error(err, "Failed to delete associated Programs")
		return ctrl.Result{}, err
	}

	// Clean up instance finalizers now that Stacks are gone.
	// This must happen BEFORE deleting the CRD, otherwise we lose access to the instances.
	if err := r.cleanupInstanceFinalizers(ctx, template); err != nil {
		log.Error(err, "Failed to clean up instance finalizers")
		// Continue anyway - instances will be orphaned but we need to complete cleanup
	}

	// Stop the informer
	r.cleanupInformer(fmt.Sprintf("%s/%s", template.Namespace, template.Name))

	// Delete the generated CRD
	if template.Status.CRD != nil && template.Status.CRD.Name != "" {
		if err := r.deleteCRD(ctx, template.Status.CRD.Name); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to delete CRD")
				return ctrl.Result{}, err
			}
		}
		log.Info("Deleted generated CRD", "name", template.Status.CRD.Name)
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(template, TemplateFinalizerName)
	if err := r.Update(ctx, template); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// listPendingStacks returns a list of Stack names that are still being destroyed.
func (r *TemplateReconciler) listPendingStacks(ctx context.Context, template *pulumiv1alpha1.Template) ([]string, error) {
	// List all Stacks created by this Template
	stackList := &pulumiv1.StackList{}
	if err := r.List(ctx, stackList, client.MatchingLabels{
		TemplateNameLabel:      template.Name,
		TemplateNamespaceLabel: template.Namespace,
	}); err != nil {
		return nil, err
	}

	var pending []string
	for _, stack := range stackList.Items {
		pending = append(pending, stack.Name)
	}
	return pending, nil
}

// deleteAssociatedPrograms deletes all Programs created by this Template.
func (r *TemplateReconciler) deleteAssociatedPrograms(ctx context.Context, template *pulumiv1alpha1.Template) error {
	log := log.FromContext(ctx)

	programList := &pulumiv1.ProgramList{}
	if err := r.List(ctx, programList, client.MatchingLabels{
		TemplateNameLabel:      template.Name,
		TemplateNamespaceLabel: template.Namespace,
	}); err != nil {
		return err
	}

	for _, program := range programList.Items {
		log.Info("Deleting Program", "name", program.Name)
		if err := r.Delete(ctx, &program); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// validateTemplate validates the Template spec.
func (r *TemplateReconciler) validateTemplate(template *pulumiv1alpha1.Template) error {
	// Validate CRD spec
	if template.Spec.CRD.APIVersion == "" {
		return fmt.Errorf("crd.apiVersion is required")
	}
	if template.Spec.CRD.Kind == "" {
		return fmt.Errorf("crd.kind is required")
	}

	// Validate schema
	if len(template.Spec.Schema.Spec) == 0 {
		return fmt.Errorf("schema.spec must have at least one field")
	}

	// Validate resources
	if len(template.Spec.Resources) == 0 {
		return fmt.Errorf("at least one resource must be defined")
	}

	for name, res := range template.Spec.Resources {
		if res.Type == "" {
			return fmt.Errorf("resource %q must have a type", name)
		}
	}

	return nil
}

// generateCRD generates a CRD from the Template.
func (r *TemplateReconciler) generateCRD(template *pulumiv1alpha1.Template) (*apiextensionsv1.CustomResourceDefinition, error) {
	group, version := parseAPIVersion(template.Spec.CRD.APIVersion)
	plural := getPluralName(template.Spec.CRD)
	singular := strings.ToLower(template.Spec.CRD.Kind)

	scope := apiextensionsv1.NamespaceScoped
	if template.Spec.CRD.Scope == pulumiv1alpha1.CRDScopeCluster {
		scope = apiextensionsv1.ClusterScoped
	}

	// Build OpenAPI schema from template schema
	specProperties := make(map[string]apiextensionsv1.JSONSchemaProps)
	requiredFields := []string{}

	for fieldName, field := range template.Spec.Schema.Spec {
		prop := r.schemaFieldToJSONSchemaProps(field)
		specProperties[fieldName] = prop
		if field.Required {
			requiredFields = append(requiredFields, fieldName)
		}
	}

	// Build status schema from explicit status fields
	statusProperties := make(map[string]apiextensionsv1.JSONSchemaProps)
	for fieldName, field := range template.Spec.Schema.Status {
		statusProperties[fieldName] = r.schemaFieldToJSONSchemaProps(field)
	}

	// Add output fields to status schema automatically
	// This allows outputs defined in the template to appear in the instance status
	statusProperties["outputs"] = apiextensionsv1.JSONSchemaProps{
		Type:                   "object",
		Description:            "Outputs from the Pulumi stack",
		XPreserveUnknownFields: ptrBool(true),
	}

	// Add standard status fields
	statusProperties["conditions"] = apiextensionsv1.JSONSchemaProps{
		Type: "array",
		Items: &apiextensionsv1.JSONSchemaPropsOrArray{
			Schema: &apiextensionsv1.JSONSchemaProps{
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"type":               {Type: "string"},
					"status":             {Type: "string"},
					"lastTransitionTime": {Type: "string", Format: "date-time"},
					"reason":             {Type: "string"},
					"message":            {Type: "string"},
				},
			},
		},
	}
	statusProperties["stackRef"] = apiextensionsv1.JSONSchemaProps{
		Type:        "string",
		Description: "Reference to the underlying Stack CR",
	}
	statusProperties["lastUpdate"] = apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"state":      {Type: "string"},
			"generation": {Type: "integer", Format: "int64"},
			"startTime":  {Type: "string", Format: "date-time"},
			"endTime":    {Type: "string", Format: "date-time"},
			"message":    {Type: "string"},
		},
	}
	statusProperties["observedGeneration"] = apiextensionsv1.JSONSchemaProps{
		Type:   "integer",
		Format: "int64",
	}
	statusProperties["ready"] = apiextensionsv1.JSONSchemaProps{
		Type:        "boolean",
		Description: "True when the instance is ready and the underlying Stack has succeeded",
	}

	// Build printer columns
	printerColumns := []apiextensionsv1.CustomResourceColumnDefinition{
		{
			Name:     "Ready",
			Type:     "string",
			JSONPath: ".status.conditions[?(@.type=='Ready')].status",
		},
		{
			Name:     "Age",
			Type:     "date",
			JSONPath: ".metadata.creationTimestamp",
		},
	}

	// Add custom printer columns from the template
	for _, col := range template.Spec.CRD.PrinterColumns {
		printerColumns = append(printerColumns, apiextensionsv1.CustomResourceColumnDefinition{
			Name:     col.Name,
			Type:     col.Type,
			JSONPath: col.JSONPath,
			Priority: col.Priority,
		})
	}

	// Add printer columns for outputs defined in the template
	// This makes outputs visible when running `kubectl get`
	for outputName := range template.Spec.Outputs {
		// Check if a printer column with this name already exists
		exists := false
		for _, col := range printerColumns {
			if strings.EqualFold(col.Name, outputName) {
				exists = true
				break
			}
		}
		if !exists {
			printerColumns = append(printerColumns, apiextensionsv1.CustomResourceColumnDefinition{
				Name:     outputName,
				Type:     "string", // Use string as default since outputs can be any type
				JSONPath: fmt.Sprintf(".status.outputs.%s", outputName),
				Priority: 1, // Lower priority so it shows after main columns
			})
		}
	}

	// Build categories
	categories := []string{"pulumi"}
	categories = append(categories, template.Spec.CRD.Categories...)

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", plural, group),
			Labels: map[string]string{
				GeneratedByLabel:       GeneratedByValue,
				TemplateNameLabel:      template.Name,
				TemplateNamespaceLabel: template.Namespace,
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:     plural,
				Singular:   singular,
				Kind:       template.Spec.CRD.Kind,
				ShortNames: template.Spec.CRD.ShortNames,
				Categories: categories,
			},
			Scope: scope,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    version,
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"apiVersion": {Type: "string"},
								"kind":       {Type: "string"},
								"metadata":   {Type: "object"},
								"spec": {
									Type:       "object",
									Properties: specProperties,
									Required:   requiredFields,
								},
								"status": {
									Type:       "object",
									Properties: statusProperties,
								},
							},
						},
					},
					Subresources: &apiextensionsv1.CustomResourceSubresources{
						Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
					},
					AdditionalPrinterColumns: printerColumns,
				},
			},
		},
	}

	return crd, nil
}

// schemaFieldToJSONSchemaProps converts a SchemaField to JSONSchemaProps.
func (r *TemplateReconciler) schemaFieldToJSONSchemaProps(field pulumiv1alpha1.SchemaField) apiextensionsv1.JSONSchemaProps {
	props := apiextensionsv1.JSONSchemaProps{
		Type:        string(field.Type),
		Description: field.Description,
	}

	if field.Default != nil {
		props.Default = field.Default
	}

	if len(field.Enum) > 0 {
		for _, e := range field.Enum {
			props.Enum = append(props.Enum, apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`"%s"`, e))})
		}
	}

	if field.Minimum != nil {
		min := float64(*field.Minimum)
		props.Minimum = &min
	}

	if field.Maximum != nil {
		max := float64(*field.Maximum)
		props.Maximum = &max
	}

	if field.MinLength != nil {
		props.MinLength = field.MinLength
	}

	if field.MaxLength != nil {
		props.MaxLength = field.MaxLength
	}

	if field.Pattern != "" {
		props.Pattern = field.Pattern
	}

	if field.Format != "" {
		props.Format = field.Format
	}

	// Handle array items - for now, allow any items
	if field.Type == pulumiv1alpha1.SchemaFieldTypeArray {
		props.Items = &apiextensionsv1.JSONSchemaPropsOrArray{
			Schema: &apiextensionsv1.JSONSchemaProps{
				XPreserveUnknownFields: ptrBool(true),
			},
		}
	}

	// Handle object properties - use x-preserve-unknown-fields for flexibility
	if field.Type == pulumiv1alpha1.SchemaFieldTypeObject {
		props.XPreserveUnknownFields = ptrBool(true)
	}

	return props
}

// ptrBool returns a pointer to a bool value.
func ptrBool(b bool) *bool {
	return &b
}

// registerCRD registers or updates the CRD with the API server.
func (r *TemplateReconciler) registerCRD(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, template *pulumiv1alpha1.Template) error {
	log := log.FromContext(ctx)

	existing, err := r.ApiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// Create new CRD
		log.Info("Creating new CRD", "name", crd.Name)
		_, err = r.ApiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create CRD: %w", err)
		}

		// Wait for CRD to be established
		return r.waitForCRD(ctx, crd.Name)
	}

	// Check if we own this CRD
	if existing.Labels[GeneratedByLabel] != GeneratedByValue {
		return fmt.Errorf("CRD %s exists but is not managed by pulumi-operator", crd.Name)
	}

	// Check if owned by same template
	if existing.Labels[TemplateNameLabel] != template.Name ||
		existing.Labels[TemplateNamespaceLabel] != template.Namespace {
		return fmt.Errorf("CRD %s is managed by a different Template", crd.Name)
	}

	// Update existing CRD
	log.Info("Updating existing CRD", "name", crd.Name)
	crd.ResourceVersion = existing.ResourceVersion
	_, err = r.ApiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, crd, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update CRD: %w", err)
	}

	return nil
}

// waitForCRD waits for the CRD to be established.
// It respects context cancellation and uses a configurable timeout.
func (r *TemplateReconciler) waitForCRD(ctx context.Context, name string) error {
	// Use a 30 second timeout for CRD establishment
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for CRD %s to be established: %w", name, ctx.Err())
		case <-ticker.C:
			crd, err := r.ApiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				// If context was cancelled, return the context error
				if ctx.Err() != nil {
					return fmt.Errorf("timeout waiting for CRD %s to be established: %w", name, ctx.Err())
				}
				return err
			}

			for _, cond := range crd.Status.Conditions {
				if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
					return nil
				}
			}
		}
	}
}

// deleteCRD deletes a CRD.
func (r *TemplateReconciler) deleteCRD(ctx context.Context, name string) error {
	return r.ApiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, name, metav1.DeleteOptions{})
}

// ensureInformer ensures that an informer is running for the generated CRD.
func (r *TemplateReconciler) ensureInformer(ctx context.Context, template *pulumiv1alpha1.Template) error {
	key := fmt.Sprintf("%s/%s", template.Namespace, template.Name)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if informer already exists
	if _, exists := r.informers[key]; exists {
		return nil
	}

	log := log.FromContext(ctx)
	log.Info("Starting informer for generated CRD")

	group, version := parseAPIVersion(template.Spec.CRD.APIVersion)
	plural := getPluralName(template.Spec.CRD)

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: plural,
	}

	// Create dynamic informer factory
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		r.DynamicClient,
		30*time.Second,
		metav1.NamespaceAll,
		nil,
	)

	informer := factory.ForResource(gvr).Informer()

	// Add event handlers
	// Note: We use context.Background() for async operations because the original ctx
	// from ensureInformer may be cancelled after the reconciler returns. Instance events
	// processed later need a fresh context.
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ctx := context.Background()
			r.handleInstanceAdd(ctx, template, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			old, oldOk := oldObj.(*unstructured.Unstructured)
			new, newOk := newObj.(*unstructured.Unstructured)
			// Only reconcile if spec changed (generation incremented)
			// This prevents unnecessary reconciliation on status-only changes
			if oldOk && newOk && old.GetGeneration() == new.GetGeneration() {
				return
			}
			ctx := context.Background()
			r.handleInstanceUpdate(ctx, template, oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			ctx := context.Background()
			r.handleInstanceDelete(ctx, template, obj)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	// Start the informer
	stopCh := make(chan struct{})
	r.stopChans[key] = stopCh
	r.informers[key] = informer

	go informer.Run(stopCh)

	// Wait for cache sync
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		// Stop the informer goroutine to prevent memory leak
		close(stopCh)
		delete(r.stopChans, key)
		delete(r.informers, key)
		return fmt.Errorf("failed to sync informer cache")
	}

	log.Info("Informer started and synced", "gvr", gvr)
	return nil
}

// cleanupInformer stops and removes the informer for a template.
func (r *TemplateReconciler) cleanupInformer(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if stopCh, exists := r.stopChans[key]; exists {
		close(stopCh)
		delete(r.stopChans, key)
	}
	delete(r.informers, key)
}

// handleInstanceAdd handles the addition of a new instance.
func (r *TemplateReconciler) handleInstanceAdd(ctx context.Context, template *pulumiv1alpha1.Template, obj interface{}) {
	log := log.FromContext(ctx)
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	log.Info("Instance added", "name", u.GetName(), "namespace", u.GetNamespace())

	// Trigger instance reconciliation
	r.reconcileInstance(ctx, template, u)
}

// handleInstanceUpdate handles the update of an instance.
func (r *TemplateReconciler) handleInstanceUpdate(ctx context.Context, template *pulumiv1alpha1.Template, oldObj, newObj interface{}) {
	log := log.FromContext(ctx)
	u, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	log.Info("Instance updated", "name", u.GetName(), "namespace", u.GetNamespace())

	// Trigger instance reconciliation
	r.reconcileInstance(ctx, template, u)
}

// handleInstanceDelete handles the deletion of an instance.
func (r *TemplateReconciler) handleInstanceDelete(ctx context.Context, template *pulumiv1alpha1.Template, obj interface{}) {
	log := log.FromContext(ctx)
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// Handle DeletedFinalStateUnknown
		if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			u, ok = tombstone.Obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
		} else {
			return
		}
	}
	log.Info("Instance deleted", "name", u.GetName(), "namespace", u.GetNamespace())
}

// reconcileInstance reconciles a single instance of a generated CRD.
func (r *TemplateReconciler) reconcileInstance(ctx context.Context, template *pulumiv1alpha1.Template, instance *unstructured.Unstructured) {
	// Add instance context to logger for correlation
	instanceID := fmt.Sprintf("%s/%s", instance.GetNamespace(), instance.GetName())
	log := log.FromContext(ctx).WithValues(
		"instance", instanceID,
		"instanceGeneration", instance.GetGeneration(),
	)
	ctx = logr.NewContext(ctx, log)

	log.Info("Reconciling instance")

	// Check if instance is being deleted
	if instance.GetDeletionTimestamp() != nil {
		r.handleInstanceDeletion(ctx, template, instance)
		return
	}

	// Ensure finalizer is present
	if err := r.ensureInstanceFinalizer(ctx, template, instance); err != nil {
		log.Error(err, "Failed to ensure finalizer on instance")
		return
	}

	// Get instance spec
	spec, found, err := unstructured.NestedMap(instance.Object, "spec")
	if err != nil {
		log.Error(err, "Error reading instance spec")
		r.updateInstanceStatus(ctx, template, instance, nil, "SpecError", err.Error())
		return
	}
	if !found {
		log.Info("Instance spec not found", "name", instance.GetName())
		r.updateInstanceStatus(ctx, template, instance, nil, "SpecMissing", "Instance spec field not found")
		return
	}

	// Generate Program name - this becomes the Pulumi project name
	// When organization is specified, use project name from stackConfig
	// Otherwise use template-instance format
	programName := r.generateProgramName(template, instance)
	stackName := fmt.Sprintf("%s-%s", template.Name, instance.GetName())

	// Create or update Program CR
	program, err := r.reconcileProgram(ctx, template, instance, programName, spec)
	if err != nil {
		log.Error(err, "Failed to reconcile Program")
		r.updateInstanceStatus(ctx, template, instance, nil, "ProgramFailed", err.Error())
		return
	}

	// Create or update Stack CR
	stack, err := r.reconcileStack(ctx, template, instance, stackName, program)
	if err != nil {
		log.Error(err, "Failed to reconcile Stack")
		r.updateInstanceStatus(ctx, template, instance, nil, "StackFailed", err.Error())
		return
	}

	// Update instance status from Stack
	r.syncInstanceStatusFromStack(ctx, template, instance, stack)
}

// ensureInstanceFinalizer ensures the instance has a finalizer for cleanup.
func (r *TemplateReconciler) ensureInstanceFinalizer(ctx context.Context, template *pulumiv1alpha1.Template, instance *unstructured.Unstructured) error {
	finalizers := instance.GetFinalizers()
	for _, f := range finalizers {
		if f == InstanceFinalizerName {
			return nil
		}
	}

	// Add finalizer
	finalizers = append(finalizers, InstanceFinalizerName)
	instance.SetFinalizers(finalizers)

	group, version := parseAPIVersion(template.Spec.CRD.APIVersion)
	plural := getPluralName(template.Spec.CRD)
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: plural}

	_, err := r.DynamicClient.Resource(gvr).Namespace(instance.GetNamespace()).Update(ctx, instance, metav1.UpdateOptions{})
	return err
}

// generateProgramName generates the Program CR name for an instance.
// This is important because the Program CR name becomes the Pulumi project name in Pulumi.yaml.
// The name must be unique per instance and match what's used in the Stack's org/project/stack format.
func (r *TemplateReconciler) generateProgramName(template *pulumiv1alpha1.Template, instance *unstructured.Unstructured) string {
	// The Program CR name becomes the Pulumi project name.
	// Format: {project-prefix}-{instance-name}
	// where project-prefix defaults to template name or can be specified in stackConfig.project
	projectPrefix := template.Name
	if template.Spec.StackConfig != nil && template.Spec.StackConfig.Project != "" {
		projectPrefix = template.Spec.StackConfig.Project
	}
	return fmt.Sprintf("%s-%s", projectPrefix, instance.GetName())
}

// handleInstanceDeletion handles cleanup when an instance is deleted.
func (r *TemplateReconciler) handleInstanceDeletion(ctx context.Context, template *pulumiv1alpha1.Template, instance *unstructured.Unstructured) {
	log := log.FromContext(ctx)
	log.Info("Handling instance deletion", "name", instance.GetName())

	// Check if we should destroy resources
	destroyOnDelete := true
	if template.Spec.Lifecycle != nil {
		destroyOnDelete = template.Spec.Lifecycle.DestroyOnDelete
	}

	stackName := fmt.Sprintf("%s-%s", template.Name, instance.GetName())
	programName := r.generateProgramName(template, instance)

	if destroyOnDelete {
		// Check if Stack exists and trigger destroy
		stack := &pulumiv1.Stack{}
		err := r.Get(ctx, types.NamespacedName{Name: stackName, Namespace: instance.GetNamespace()}, stack)
		if err == nil {
			// Stack exists - check if destroy is complete
			if stack.DeletionTimestamp == nil {
				// Stack not yet marked for deletion, delete it now
				// The Stack's finalizer will handle the destroy operation
				log.Info("Deleting Stack to trigger destroy", "stack", stackName)
				if err := r.Delete(ctx, stack); err != nil && !apierrors.IsNotFound(err) {
					log.Error(err, "Failed to delete Stack")
					return
				}
				// Don't remove finalizer yet - wait for Stack to be fully deleted
				log.Info("Waiting for Stack destroy to complete", "stack", stackName)
				return
			}
			// Stack is being deleted but still exists - wait for it to complete
			log.Info("Stack still being destroyed, waiting", "stack", stackName)
			return
		} else if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get Stack")
			return
		}
		// Stack is gone, now we can clean up the Program
		log.Info("Stack destroyed, cleaning up Program", "stack", stackName)
	} else {
		// Just delete Stack without destroying resources
		stack := &pulumiv1.Stack{}
		if err := r.Get(ctx, types.NamespacedName{Name: stackName, Namespace: instance.GetNamespace()}, stack); err == nil {
			// Use Patch for atomic update to prevent race condition where Stack controller
			// might process the deletion before the update is persisted.
			patch := client.MergeFrom(stack.DeepCopy())
			stack.Spec.DestroyOnFinalize = false
			if err := r.Patch(ctx, stack, patch); err != nil {
				log.Error(err, "Failed to patch Stack to disable DestroyOnFinalize")
				return
			}
			// Now safe to delete - the patch ensures DestroyOnFinalize is false
			if err := r.Delete(ctx, stack); err != nil && !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to delete Stack")
				return
			}
		} else if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get Stack")
			return
		}
	}

	// Delete Program only after Stack is fully gone
	program := &pulumiv1.Program{}
	if err := r.Get(ctx, types.NamespacedName{Name: programName, Namespace: instance.GetNamespace()}, program); err == nil {
		if err := r.Delete(ctx, program); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to delete Program")
			return
		}
	}

	// Remove finalizer from instance
	if err := r.removeInstanceFinalizer(ctx, template, instance); err != nil {
		log.Error(err, "Failed to remove instance finalizer, will retry on next event")
		// Don't return - the informer will trigger another deletion attempt
	}
}

// removeInstanceFinalizer removes the finalizer from the instance.
// Returns an error if the finalizer removal fails.
func (r *TemplateReconciler) removeInstanceFinalizer(ctx context.Context, template *pulumiv1alpha1.Template, instance *unstructured.Unstructured) error {
	finalizers := instance.GetFinalizers()
	newFinalizers := make([]string, 0, len(finalizers))
	for _, f := range finalizers {
		if f != InstanceFinalizerName {
			newFinalizers = append(newFinalizers, f)
		}
	}
	instance.SetFinalizers(newFinalizers)

	group, version := parseAPIVersion(template.Spec.CRD.APIVersion)
	plural := getPluralName(template.Spec.CRD)
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: plural}

	if _, err := r.DynamicClient.Resource(gvr).Namespace(instance.GetNamespace()).Update(ctx, instance, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to remove finalizer from instance %s/%s: %w", instance.GetNamespace(), instance.GetName(), err)
	}
	return nil
}

// cleanupInstanceFinalizers checks for instances that are being deleted and whose Stacks
// have been fully deleted (destroy completed). For such instances, it removes the finalizer
// to allow them to be garbage collected. This is called when a Stack deletion triggers
// a Template reconciliation via the Stack watch.
func (r *TemplateReconciler) cleanupInstanceFinalizers(ctx context.Context, template *pulumiv1alpha1.Template) error {
	log := log.FromContext(ctx)

	// Check if CRD exists
	if template.Status.CRD == nil || template.Status.CRD.Name == "" {
		return nil
	}

	group, version := parseAPIVersion(template.Spec.CRD.APIVersion)
	plural := getPluralName(template.Spec.CRD)
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: plural}

	// List all instances
	instances, err := r.DynamicClient.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	for _, instance := range instances.Items {
		// Only process instances that are being deleted
		if instance.GetDeletionTimestamp() == nil {
			continue
		}

		// Check if instance has our finalizer
		hasFinalizer := false
		for _, f := range instance.GetFinalizers() {
			if f == InstanceFinalizerName {
				hasFinalizer = true
				break
			}
		}
		if !hasFinalizer {
			continue
		}

		// Check if the Stack for this instance still exists
		stackName := fmt.Sprintf("%s-%s", template.Name, instance.GetName())
		stack := &pulumiv1.Stack{}
		err := r.Get(ctx, types.NamespacedName{Name: stackName, Namespace: instance.GetNamespace()}, stack)
		if err == nil {
			// Stack still exists - skip this instance
			continue
		}
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to check Stack status", "stack", stackName)
			continue
		}

		// Stack is gone - check if Program needs to be deleted
		programName := r.generateProgramName(template, &instance)
		program := &pulumiv1.Program{}
		if err := r.Get(ctx, types.NamespacedName{Name: programName, Namespace: instance.GetNamespace()}, program); err == nil {
			log.Info("Deleting orphaned Program", "program", programName)
			if err := r.Delete(ctx, program); err != nil && !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to delete Program", "program", programName)
			}
		}

		// Stack is gone, remove the finalizer
		log.Info("Stack destroyed, removing instance finalizer",
			"instance", instance.GetName(),
			"namespace", instance.GetNamespace())
		if err := r.removeInstanceFinalizer(ctx, template, &instance); err != nil {
			// Log error but continue - the next Stack watch event will trigger another cleanup attempt
			log.Error(err, "Failed to remove instance finalizer, will retry on next reconciliation",
				"instance", instance.GetName(),
				"namespace", instance.GetNamespace())
		}
	}

	return nil
}

// syncInstanceStatusesFromStacks syncs instance statuses from their associated Stacks.
// This is called when a Stack update triggers a Template reconciliation (via the Stack watch).
// It finds all instances and syncs their status from the corresponding Stack.
func (r *TemplateReconciler) syncInstanceStatusesFromStacks(ctx context.Context, template *pulumiv1alpha1.Template) error {
	log := log.FromContext(ctx)

	// Check if CRD exists
	if template.Status.CRD == nil || template.Status.CRD.Name == "" {
		return nil
	}

	group, version := parseAPIVersion(template.Spec.CRD.APIVersion)
	plural := getPluralName(template.Spec.CRD)
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: plural}

	// List all instances
	instances, err := r.DynamicClient.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	for _, instance := range instances.Items {
		// Skip instances that are being deleted
		if instance.GetDeletionTimestamp() != nil {
			continue
		}

		// Find the Stack for this instance
		stackName := fmt.Sprintf("%s-%s", template.Name, instance.GetName())
		stack := &pulumiv1.Stack{}
		err := r.Get(ctx, types.NamespacedName{Name: stackName, Namespace: instance.GetNamespace()}, stack)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Stack doesn't exist yet - skip
				continue
			}
			log.Error(err, "Failed to get Stack for instance", "stack", stackName, "instance", instance.GetName())
			continue
		}

		// Sync instance status from Stack
		r.syncInstanceStatusFromStack(ctx, template, &instance, stack)
	}

	return nil
}

// reconcileProgram creates or updates the Program CR for an instance using Server-Side Apply.
func (r *TemplateReconciler) reconcileProgram(ctx context.Context, template *pulumiv1alpha1.Template, instance *unstructured.Unstructured, programName string, instanceSpec map[string]interface{}) (*pulumiv1.Program, error) {
	log := log.FromContext(ctx)

	// Build the Program spec by rendering template resources with instance values
	programSpec := r.renderProgramSpec(template, instance, instanceSpec)

	// Build Program for Server-Side Apply
	program := &pulumiv1.Program{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pulumiv1.GroupVersion.String(),
			Kind:       "Program",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      programName,
			Namespace: instance.GetNamespace(),
			Labels: map[string]string{
				GeneratedByLabel:       GeneratedByValue,
				TemplateNameLabel:      template.Name,
				TemplateNamespaceLabel: template.Namespace,
				InstanceNameLabel:      instance.GetName(),
				InstanceNamespaceLabel: instance.GetNamespace(),
			},
		},
		Program: programSpec,
	}

	// Use Server-Side Apply to create or update the Program
	log.Info("Applying Program", "name", programName)
	if err := r.Patch(ctx, program, client.Apply, client.FieldOwner(TemplateFieldManager)); err != nil {
		return nil, fmt.Errorf("failed to apply Program: %w", err)
	}

	return program, nil
}

// renderProgramSpec renders the template resources into a ProgramSpec.
func (r *TemplateReconciler) renderProgramSpec(template *pulumiv1alpha1.Template, instance *unstructured.Unstructured, instanceSpec map[string]interface{}) pulumiv1.ProgramSpec {
	// We don't use Pulumi configuration because we substitute values directly into resources.
	// This avoids type mismatch issues with Pulumi YAML's limited config types (String, Number, List).

	// Copy resources from template, replacing schema references with actual values
	resources := make(map[string]pulumiv1.Resource)
	for name, res := range template.Spec.Resources {
		renderedRes := pulumiv1.Resource{
			Type:    res.Type,
			Options: res.Options,
			Get:     res.Get,
		}

		// Render properties by replacing ${schema.spec.X} with ${X}
		// and ${schema.metadata.X} with ${instanceX}
		if res.Properties != nil {
			renderedProps := make(map[string]pulumiv1.Expression)
			for propName, propValue := range res.Properties {
				renderedProps[propName] = r.renderExpression(propValue, instance, instanceSpec)
			}
			renderedRes.Properties = renderedProps
		}

		resources[name] = renderedRes
	}

	// Copy variables
	variables := make(map[string]pulumiv1.Expression)
	for name, expr := range template.Spec.Variables {
		variables[name] = r.renderExpression(expr, instance, instanceSpec)
	}

	// Copy outputs
	outputs := make(map[string]pulumiv1.Expression)
	for name, expr := range template.Spec.Outputs {
		outputs[name] = r.renderExpression(expr, instance, instanceSpec)
	}

	return pulumiv1.ProgramSpec{
		Resources: resources,
		Variables: variables,
		Outputs:   outputs,
		Packages:  template.Spec.Packages,
	}
}

// renderExpression renders an expression by replacing schema references with actual values.
// This substitutes ${schema.spec.X} with the actual value from the instance spec,
// and ${schema.metadata.X} with the actual metadata values.
// It also handles complex expressions like ${schema.spec.X || schema.metadata.Y}
func (r *TemplateReconciler) renderExpression(expr pulumiv1.Expression, instance *unstructured.Unstructured, instanceSpec map[string]interface{}) pulumiv1.Expression {
	exprStr := string(expr.Raw)

	// Handle expressions with || operator (e.g., "${schema.spec.X || schema.metadata.Y}")
	// This pattern matches ${...||...} expressions
	exprStr = r.handleOrExpressions(exprStr, instance, instanceSpec)

	// Replace ${schema.metadata.name} with instance name
	exprStr = strings.ReplaceAll(exprStr, "${schema.metadata.name}", instance.GetName())
	exprStr = strings.ReplaceAll(exprStr, "${schema.metadata.namespace}", instance.GetNamespace())

	// Replace ${schema.spec.X} with actual values from instance spec
	// This handles the case where the entire expression is a schema reference
	for fieldName, fieldValue := range instanceSpec {
		placeholder := fmt.Sprintf("${schema.spec.%s}", fieldName)
		quotedPlaceholder := fmt.Sprintf(`"%s"`, placeholder)

		// Convert the value to appropriate JSON representation
		valueStr := r.valueToExpressionString(fieldValue)

		// Check if this is a standalone expression (just the placeholder)
		// vs embedded in a larger string
		if exprStr == quotedPlaceholder {
			// Standalone - replace the entire expression with the value
			exprStr = valueStr
		} else if strings.Contains(exprStr, quotedPlaceholder) {
			// Quoted placeholder embedded in larger structure (e.g., JSON object)
			// Replace the quoted placeholder with the properly typed JSON value
			// This handles cases like {"instanceCount": "${schema.spec.instanceCount}"}
			// becoming {"instanceCount": 1} instead of {"instanceCount": "1"}
			exprStr = strings.ReplaceAll(exprStr, quotedPlaceholder, valueStr)
		} else {
			// Unquoted placeholder embedded in string - do string interpolation
			exprStr = strings.ReplaceAll(exprStr, placeholder, fmt.Sprintf("%v", fieldValue))
		}
	}

	return pulumiv1.Expression{Raw: []byte(exprStr)}
}

// handleOrExpressions handles expressions like "${schema.spec.X || schema.metadata.Y}"
// by evaluating each part and returning the first non-empty value.
func (r *TemplateReconciler) handleOrExpressions(exprStr string, instance *unstructured.Unstructured, instanceSpec map[string]interface{}) string {
	// Find all ${...||...} patterns
	// This regex matches ${...} where ... contains ||
	for {
		start := strings.Index(exprStr, "${")
		if start == -1 {
			break
		}
		end := strings.Index(exprStr[start:], "}")
		if end == -1 {
			break
		}
		end += start

		innerExpr := exprStr[start+2 : end]
		if !strings.Contains(innerExpr, "||") {
			// Not an OR expression, skip past this ${} and continue
			// Find next ${ after this one
			nextStart := strings.Index(exprStr[end+1:], "${")
			if nextStart == -1 {
				break
			}
			exprStr = exprStr[:end+1] + r.handleOrExpressions(exprStr[end+1:], instance, instanceSpec)
			break
		}

		// Split by || and evaluate each part
		parts := strings.Split(innerExpr, "||")
		var resolvedValue string
		for _, part := range parts {
			part = strings.TrimSpace(part)
			value := r.resolveSchemaReference(part, instance, instanceSpec)
			if value != "" {
				resolvedValue = value
				break
			}
		}

		// Replace the entire ${...||...} with the resolved value
		fullExpr := exprStr[start : end+1]
		if resolvedValue != "" {
			exprStr = strings.Replace(exprStr, fullExpr, resolvedValue, 1)
		} else {
			// No value found, leave as Pulumi expression without schema refs
			// This shouldn't happen in normal cases
			exprStr = strings.Replace(exprStr, fullExpr, instance.GetName(), 1)
		}
	}
	return exprStr
}

// resolveSchemaReference resolves a single schema reference like "schema.spec.X" or "schema.metadata.name"
func (r *TemplateReconciler) resolveSchemaReference(ref string, instance *unstructured.Unstructured, instanceSpec map[string]interface{}) string {
	if strings.HasPrefix(ref, "schema.metadata.name") {
		return instance.GetName()
	}
	if strings.HasPrefix(ref, "schema.metadata.namespace") {
		return instance.GetNamespace()
	}
	if strings.HasPrefix(ref, "schema.spec.") {
		fieldName := strings.TrimPrefix(ref, "schema.spec.")
		if value, ok := instanceSpec[fieldName]; ok {
			// Return string value, empty string if nil
			if value == nil {
				return ""
			}
			switch v := value.(type) {
			case string:
				return v
			default:
				return fmt.Sprintf("%v", v)
			}
		}
	}
	return ""
}

// valueToExpressionString converts a Go value to a JSON expression string.
func (r *TemplateReconciler) valueToExpressionString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf(`"%s"`, v)
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		// For complex types, marshal to JSON
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf(`"%v"`, v)
		}
		return string(jsonBytes)
	}
}

// reconcileStack creates or updates the Stack CR for an instance using Server-Side Apply.
func (r *TemplateReconciler) reconcileStack(ctx context.Context, template *pulumiv1alpha1.Template, instance *unstructured.Unstructured, stackName string, program *pulumiv1.Program) (*pulumiv1.Stack, error) {
	log := log.FromContext(ctx)

	// Build Stack spec
	stackSpec := r.buildStackSpec(template, instance, program)

	// Build Stack for Server-Side Apply
	stack := &pulumiv1.Stack{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pulumiv1.GroupVersion.String(),
			Kind:       "Stack",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      stackName,
			Namespace: instance.GetNamespace(),
			Labels: map[string]string{
				GeneratedByLabel:       GeneratedByValue,
				TemplateNameLabel:      template.Name,
				TemplateNamespaceLabel: template.Namespace,
				InstanceNameLabel:      instance.GetName(),
				InstanceNamespaceLabel: instance.GetNamespace(),
			},
		},
		Spec: stackSpec,
	}

	// Use Server-Side Apply to create or update the Stack
	log.Info("Applying Stack", "name", stackName)
	if err := r.Patch(ctx, stack, client.Apply, client.FieldOwner(TemplateFieldManager)); err != nil {
		return nil, fmt.Errorf("failed to apply Stack: %w", err)
	}

	return stack, nil
}

// buildStackSpec builds a Stack spec from the template and instance.
func (r *TemplateReconciler) buildStackSpec(template *pulumiv1alpha1.Template, instance *unstructured.Unstructured, program *pulumiv1.Program) shared.StackSpec {
	// Build stack name: if organization is specified, use org/project/stack format
	// Otherwise, use a simple name and let Pulumi use the default org from the token
	// IMPORTANT: The project name must match the Program CR name, which becomes the Pulumi project name in Pulumi.yaml
	stackName := fmt.Sprintf("%s-%s-%s", template.Name, instance.GetNamespace(), instance.GetName())

	if template.Spec.StackConfig != nil {
		if template.Spec.StackConfig.Organization != "" {
			// Organization specified - use fully qualified name
			// The project name is the Program CR name: {project-prefix}-{instance-name}
			stackName = fmt.Sprintf("%s/%s/%s", template.Spec.StackConfig.Organization, program.Name, instance.GetName())
		} else if template.Spec.StackConfig.Project != "" {
			// Project specified without org - use project/stack format
			// The project name is the Program CR name
			stackName = fmt.Sprintf("%s/%s", program.Name, instance.GetName())
		}
	}

	stackSpec := shared.StackSpec{
		// Reference the Program
		ProgramRef: &shared.ProgramReference{
			Name: program.Name,
		},
		// Stack name - can be simple name or fully qualified (org/project/stack)
		Stack: stackName,
	}

	// Apply lifecycle settings
	if template.Spec.Lifecycle != nil {
		stackSpec.DestroyOnFinalize = template.Spec.Lifecycle.DestroyOnDelete
		stackSpec.Refresh = template.Spec.Lifecycle.RefreshBeforeUpdate
		stackSpec.RetryOnUpdateConflict = template.Spec.Lifecycle.RetryOnUpdateConflict
	}

	// Apply stack config from template
	if template.Spec.StackConfig != nil {
		if template.Spec.StackConfig.ServiceAccountName != "" {
			stackSpec.ServiceAccountName = template.Spec.StackConfig.ServiceAccountName
		}
		if template.Spec.StackConfig.Backend != "" {
			stackSpec.Backend = template.Spec.StackConfig.Backend
		}
		if template.Spec.StackConfig.SecretsProvider != "" {
			stackSpec.SecretsProvider = template.Spec.StackConfig.SecretsProvider
		}
		if template.Spec.StackConfig.EnvRefs != nil {
			stackSpec.EnvRefs = template.Spec.StackConfig.EnvRefs
		}
		if len(template.Spec.StackConfig.Envs) > 0 {
			stackSpec.Envs = template.Spec.StackConfig.Envs
		}
		if len(template.Spec.StackConfig.Environment) > 0 {
			stackSpec.Environment = template.Spec.StackConfig.Environment
		}
		if template.Spec.StackConfig.WorkspaceTemplate != nil {
			stackSpec.WorkspaceTemplate = template.Spec.StackConfig.WorkspaceTemplate
		}
		if template.Spec.StackConfig.ResyncFrequencySeconds != nil {
			stackSpec.ResyncFrequencySeconds = *template.Spec.StackConfig.ResyncFrequencySeconds
		}
	}

	return stackSpec
}

// syncInstanceStatusFromStack syncs the instance status from the Stack status.
func (r *TemplateReconciler) syncInstanceStatusFromStack(ctx context.Context, template *pulumiv1alpha1.Template, instance *unstructured.Unstructured, stack *pulumiv1.Stack) {
	log := log.FromContext(ctx)

	group, version := parseAPIVersion(template.Spec.CRD.APIVersion)
	plural := getPluralName(template.Spec.CRD)
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: plural}

	// Re-fetch the instance to get the latest resourceVersion to avoid conflicts
	latestInstance, err := r.DynamicClient.Resource(gvr).Namespace(instance.GetNamespace()).Get(ctx, instance.GetName(), metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Failed to get latest instance for status update")
		return
	}

	// Build status from Stack
	status := make(map[string]interface{})

	// Add stackRef
	status["stackRef"] = stack.Name

	// Add observedGeneration
	status["observedGeneration"] = latestInstance.GetGeneration()

	// Map Stack conditions to instance conditions and compute ready status
	conditions := []interface{}{}
	ready := false
	for _, cond := range stack.Status.Conditions {
		conditions = append(conditions, map[string]interface{}{
			"type":               cond.Type,
			"status":             string(cond.Status),
			"lastTransitionTime": cond.LastTransitionTime.Format(time.RFC3339),
			"reason":             cond.Reason,
			"message":            cond.Message,
		})
		// Check if the Ready condition is True
		if cond.Type == "Ready" && cond.Status == metav1.ConditionTrue {
			ready = true
		}
	}
	status["conditions"] = conditions
	status["ready"] = ready

	// Add lastUpdate from Stack
	if stack.Status.LastUpdate != nil {
		status["lastUpdate"] = map[string]interface{}{
			"state":      string(stack.Status.LastUpdate.State),
			"generation": stack.Status.LastUpdate.Generation,
			"message":    stack.Status.LastUpdate.Message,
		}
	}

	// Map ALL outputs from Stack to instance status
	// 1. Create an "outputs" object containing all stack outputs
	// 2. Also map outputs defined in template.Spec.Outputs as top-level status fields for convenience
	// 3. For secret outputs (showing as "[secret]"), try to read actual values from outputs Secret
	outputsMap := make(map[string]interface{})

	// First, try to get actual secret values from the outputs Secret
	secretOutputs := r.getOutputsFromSecret(ctx, stack)

	if stack.Status.Outputs != nil {
		for outputName, outputValue := range stack.Status.Outputs {
			var value interface{}
			if err := json.Unmarshal(outputValue.Raw, &value); err == nil {
				// Check if this is a secret placeholder and we have the actual value
				if strVal, ok := value.(string); ok && strVal == "[secret]" {
					if secretVal, exists := secretOutputs[outputName]; exists {
						outputsMap[outputName] = secretVal
					} else {
						outputsMap[outputName] = value
					}
				} else {
					outputsMap[outputName] = value
				}
			}
		}
	}

	// Also add any outputs from secret that aren't in Stack outputs
	for outputName, outputValue := range secretOutputs {
		if _, exists := outputsMap[outputName]; !exists {
			outputsMap[outputName] = outputValue
		}
	}

	if len(outputsMap) > 0 {
		status["outputs"] = outputsMap
	}

	// Also map outputs defined in template.Spec.Outputs as top-level status fields
	for outputName := range template.Spec.Outputs {
		if outputValue, ok := outputsMap[outputName]; ok {
			status[outputName] = outputValue
		}
	}

	// Also map any status fields defined in the schema
	for fieldName := range template.Spec.Schema.Status {
		if _, exists := status[fieldName]; !exists {
			// Check if there's an output with this name
			if stack.Status.Outputs != nil {
				if outputValue, ok := stack.Status.Outputs[fieldName]; ok {
					var value interface{}
					if err := json.Unmarshal(outputValue.Raw, &value); err == nil {
						status[fieldName] = value
					}
				}
			}
		}
	}

	// Update instance status
	if err := unstructured.SetNestedField(latestInstance.Object, status, "status"); err != nil {
		log.Error(err, "Failed to set instance status")
		return
	}

	if _, err := r.DynamicClient.Resource(gvr).Namespace(latestInstance.GetNamespace()).UpdateStatus(ctx, latestInstance, metav1.UpdateOptions{}); err != nil {
		log.Error(err, "Failed to update instance status")
	}
}

// getOutputsFromSecret reads output values from the Stack's outputs Secret.
// This is used to get actual values for secret outputs that show as "[secret]" in Stack status.
func (r *TemplateReconciler) getOutputsFromSecret(ctx context.Context, stack *pulumiv1.Stack) map[string]interface{} {
	log := log.FromContext(ctx)
	outputs := make(map[string]interface{})

	// The outputs secret name is based on the last update name
	if stack.Status.LastUpdate == nil || stack.Status.LastUpdate.Name == "" {
		return outputs
	}

	secretName := stack.Status.LastUpdate.Name + "-stack-outputs"

	// Get the secret
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: stack.Namespace}, secret); err != nil {
		// Secret may not exist or we may not have access
		log.V(1).Info("Could not read outputs secret", "secretName", secretName, "error", err)
		return outputs
	}

	// Extract outputs from secret data
	for key, value := range secret.Data {
		// Try to parse as JSON first (for complex types)
		var jsonValue interface{}
		if err := json.Unmarshal(value, &jsonValue); err == nil {
			outputs[key] = jsonValue
		} else {
			// Treat as string
			outputs[key] = string(value)
		}
	}

	return outputs
}

// updateInstanceStatus updates the instance status with error information.
func (r *TemplateReconciler) updateInstanceStatus(ctx context.Context, template *pulumiv1alpha1.Template, instance *unstructured.Unstructured, stack *pulumiv1.Stack, reason, message string) {
	log := log.FromContext(ctx)

	status := make(map[string]interface{})
	status["observedGeneration"] = instance.GetGeneration()

	// Set error condition
	conditions := []interface{}{
		map[string]interface{}{
			"type":               "Ready",
			"status":             "False",
			"lastTransitionTime": time.Now().Format(time.RFC3339),
			"reason":             reason,
			"message":            message,
		},
	}
	status["conditions"] = conditions

	if stack != nil {
		status["stackRef"] = stack.Name
	}

	if err := unstructured.SetNestedField(instance.Object, status, "status"); err != nil {
		log.Error(err, "Failed to set instance status")
		return
	}

	group, version := parseAPIVersion(template.Spec.CRD.APIVersion)
	plural := getPluralName(template.Spec.CRD)
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: plural}

	if _, err := r.DynamicClient.Resource(gvr).Namespace(instance.GetNamespace()).UpdateStatus(ctx, instance, metav1.UpdateOptions{}); err != nil {
		log.Error(err, "Failed to update instance status")
	}
}

// countInstances counts the number of instances of a generated CRD.
func (r *TemplateReconciler) countInstances(ctx context.Context, template *pulumiv1alpha1.Template) (int32, error) {
	group, version := parseAPIVersion(template.Spec.CRD.APIVersion)
	plural := getPluralName(template.Spec.CRD)

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: plural,
	}

	list, err := r.DynamicClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}

	// Cap at max int32 to prevent overflow (unlikely in practice)
	count := len(list.Items)
	if count > 2147483647 {
		count = 2147483647
	}
	return int32(count), nil // #nosec G115 - capped above
}

// setCondition sets a condition on the template status.
func (r *TemplateReconciler) setCondition(template *pulumiv1alpha1.Template, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: template.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	template.Status.SetCondition(condition)
}

// updateStatusWithConflictHandling updates the template status, handling conflict errors gracefully.
// Returns (shouldRequeue, error). If shouldRequeue is true, the caller should return Requeue: true.
// Conflict errors are logged at debug level and result in a requeue without error.
func (r *TemplateReconciler) updateStatusWithConflictHandling(ctx context.Context, template *pulumiv1alpha1.Template) (bool, error) {
	if err := r.Status().Update(ctx, template); err != nil {
		if apierrors.IsConflict(err) {
			// Conflict errors are expected when multiple events trigger reconciliation
			// simultaneously (e.g., Stack status changes). Just requeue silently.
			log.FromContext(ctx).V(1).Info("Conflict updating status, will retry", "error", err)
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// parseAPIVersion parses an API version string into group and version.
func parseAPIVersion(apiVersion string) (group, version string) {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}

// getPluralName returns the plural name for the CRD.
// If an explicit plural is specified in the CRD spec, it is used.
// Otherwise, it defaults to lowercase(kind) + "s".
func getPluralName(crdSpec pulumiv1alpha1.CRDSpec) string {
	if crdSpec.Plural != "" {
		return crdSpec.Plural
	}
	return strings.ToLower(crdSpec.Kind) + "s"
}

// templateContext holds pre-computed values for a template to avoid repeated parsing.
// This reduces redundant computations during reconciliation.
type templateContext struct {
	template *pulumiv1alpha1.Template
	group    string
	version  string
	plural   string
	gvr      schema.GroupVersionResource
}

// newTemplateContext creates a new templateContext with pre-computed GVR values.
func newTemplateContext(template *pulumiv1alpha1.Template) *templateContext {
	group, version := parseAPIVersion(template.Spec.CRD.APIVersion)
	plural := getPluralName(template.Spec.CRD)
	return &templateContext{
		template: template,
		group:    group,
		version:  version,
		plural:   plural,
		gvr:      schema.GroupVersionResource{Group: group, Version: version, Resource: plural},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *TemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Track metrics about Template resources.
	templateInformer, err := mgr.GetCache().GetInformer(context.Background(), new(pulumiv1alpha1.Template))
	if err != nil {
		return err
	}
	if _, err = templateInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    newTemplateCallback,
		UpdateFunc: updateTemplateCallback,
		DeleteFunc: deleteTemplateCallback,
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(TemplateControllerName).
		For(&pulumiv1alpha1.Template{}).
		// Watch Stacks that were created by Templates.
		// This triggers Template reconciliation to:
		// 1. Sync instance status when Stack completes (Ready condition changes)
		// 2. Clean up instance finalizers when Stack is deleted (after destroy completes)
		Watches(
			&pulumiv1.Stack{},
			handler.EnqueueRequestsFromMapFunc(r.stackToTemplateMapper),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool { return false },
				UpdateFunc: func(e event.UpdateEvent) bool {
					// Trigger on Ready condition changes to sync instance status
					newStack, newOk := e.ObjectNew.(*pulumiv1.Stack)
					if !newOk {
						return false
					}
					// Only trigger for Stacks created by Templates
					if newStack.Labels[TemplateNameLabel] == "" {
						return false
					}
					// Always trigger for Template-managed Stacks - the reconciler will handle filtering
					ctrl.Log.Info("Stack update detected, triggering Template reconciliation",
						"stack", newStack.Name,
						"namespace", newStack.Namespace)
					return true
				},
				DeleteFunc:  func(e event.DeleteEvent) bool { return true },
				GenericFunc: func(e event.GenericEvent) bool { return false },
			}),
		).
		// Note: We don't use WithEventFilter here because it would also filter Stack watch events.
		// The Stack watch needs to trigger on status-only changes (Ready condition), but
		// GenerationChangedPredicate would filter those out since status updates don't change generation.
		// Instead, we use predicates directly on each watch source.
		WithOptions(controller.Options{
			// Configure rate limiter with more conservative settings for a controller
			// that creates CRDs and infrastructure resources.
			// Base delay: 1 second (instead of default 5ms)
			// Max delay: 5 minutes (instead of default 1000s)
			RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
				1*time.Second,
				5*time.Minute,
			),
		}).
		Complete(r)
}

// stackReadyConditionChanged checks if the Stack's Ready condition status changed.
// This is used to trigger instance status sync when a Stack becomes Ready.
func stackReadyConditionChanged(oldStack, newStack *pulumiv1.Stack) bool {
	getReadyStatus := func(stack *pulumiv1.Stack) metav1.ConditionStatus {
		for _, cond := range stack.Status.Conditions {
			if cond.Type == "Ready" {
				return cond.Status
			}
		}
		return metav1.ConditionUnknown
	}

	oldReady := getReadyStatus(oldStack)
	newReady := getReadyStatus(newStack)

	return oldReady != newReady
}

// stackToTemplateMapper maps a Stack to its parent Template for reconciliation.
// This is used to:
// 1. Sync instance status when Stack completes (Ready condition changes)
// 2. Clean up instance finalizers when a Stack is deleted
func (r *TemplateReconciler) stackToTemplateMapper(ctx context.Context, obj client.Object) []reconcile.Request {
	stack, ok := obj.(*pulumiv1.Stack)
	if !ok {
		return nil
	}

	// Check if this Stack was created by a Template
	templateName := stack.Labels[TemplateNameLabel]
	templateNamespace := stack.Labels[TemplateNamespaceLabel]
	if templateName == "" || templateNamespace == "" {
		return nil
	}

	log := log.FromContext(ctx)
	log.Info("Stack event, enqueueing Template for reconciliation",
		"stack", stack.Name,
		"template", templateName,
		"templateNamespace", templateNamespace)

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      templateName,
				Namespace: templateNamespace,
			},
		},
	}
}
