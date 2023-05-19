package main

import (
	"fmt"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/yaml"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

const DefaultCRDVersion = "v1.12.1"
const DefaultOperatorVersion = "v1.12.1"

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		var namespace = "default"
		if ns := config.Get(ctx, "namespace"); ns != "" {
			namespace = ns
		}
		var namespaces []string
		if err := config.GetObject(ctx, "namespaces", &namespaces); err != nil {
			return err
		}
		if len(namespaces) == 0 {
			namespaces = []string{namespace}
		}

		var crdVersion string
		if crdVersion = config.Get(ctx, "crd-version"); crdVersion == "" {
			crdVersion = DefaultCRDVersion
		}
		var operatorVersion string
		if operatorVersion = config.Get(ctx, "operator-version"); operatorVersion == "" {
			operatorVersion = DefaultOperatorVersion
		}

		var deploymentOpts []pulumi.ResourceOption
		stackCRD, err := yaml.NewConfigFile(ctx, "stackcrd", &yaml.ConfigFileArgs{
			File: fmt.Sprintf("https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/%s/deploy/crds/pulumi.com_stacks.yaml", crdVersion),
		})
		if err != nil {
			return err
		}
		programCRD, err := yaml.NewConfigFile(ctx, "programcrd", &yaml.ConfigFileArgs{
			File: fmt.Sprintf("https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/%s/deploy/crds/pulumi.com_programs.yaml", crdVersion),
		})
		if err != nil {
			return err
		}

		deploymentOpts = append(deploymentOpts, pulumi.DependsOn([]pulumi.Resource{stackCRD, programCRD}))

		for _, ns := range namespaces {
			operatorServiceAccount, err := corev1.NewServiceAccount(ctx, fmt.Sprintf("operator-service-account-%s", ns), &corev1.ServiceAccountArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Namespace: pulumi.String(ns),
				},
			})
			if err != nil {
				return err
			}

			commonObjectMeta := &metav1.ObjectMetaArgs{
				Namespace: pulumi.String(ns),
			}
			operatorRole, err := rbacv1.NewRole(ctx, fmt.Sprintf("operator-role-%s", ns), &rbacv1.RoleArgs{
				Metadata: commonObjectMeta,
				Rules: rbacv1.PolicyRuleArray{
					&rbacv1.PolicyRuleArgs{
						ApiGroups: pulumi.StringArray{
							pulumi.String(""),
						},
						Resources: pulumi.StringArray{
							pulumi.String("pods"),
							pulumi.String("services"),
							pulumi.String("services/finalizers"),
							pulumi.String("endpoints"),
							pulumi.String("persistentvolumeclaims"),
							pulumi.String("events"),
							pulumi.String("configmaps"),
							pulumi.String("secrets"),
						},
						Verbs: pulumi.StringArray{
							pulumi.String("create"),
							pulumi.String("delete"),
							pulumi.String("get"),
							pulumi.String("list"),
							pulumi.String("patch"),
							pulumi.String("update"),
							pulumi.String("watch"),
						},
					},
					&rbacv1.PolicyRuleArgs{
						ApiGroups: pulumi.StringArray{
							pulumi.String("apps"),
						},
						Resources: pulumi.StringArray{
							pulumi.String("deployments"),
							pulumi.String("daemonsets"),
							pulumi.String("replicasets"),
							pulumi.String("statefulsets"),
						},
						Verbs: pulumi.StringArray{
							pulumi.String("create"),
							pulumi.String("delete"),
							pulumi.String("get"),
							pulumi.String("list"),
							pulumi.String("patch"),
							pulumi.String("update"),
							pulumi.String("watch"),
						},
					},
					&rbacv1.PolicyRuleArgs{
						ApiGroups: pulumi.StringArray{
							pulumi.String("monitoring.coreos.com"),
						},
						Resources: pulumi.StringArray{
							pulumi.String("servicemonitors"),
						},
						Verbs: pulumi.StringArray{
							pulumi.String("create"),
							pulumi.String("get"),
						},
					},
					&rbacv1.PolicyRuleArgs{
						ApiGroups: pulumi.StringArray{
							pulumi.String("apps"),
						},
						ResourceNames: pulumi.StringArray{
							pulumi.String("pulumi-kubernetes-operator"),
						},
						Resources: pulumi.StringArray{
							pulumi.String("deployments/finalizers"),
						},
						Verbs: pulumi.StringArray{
							pulumi.String("update"),
						},
					},
					&rbacv1.PolicyRuleArgs{
						ApiGroups: pulumi.StringArray{
							pulumi.String(""),
						},
						Resources: pulumi.StringArray{
							pulumi.String("pods"),
						},
						Verbs: pulumi.StringArray{
							pulumi.String("get"),
						},
					},
					&rbacv1.PolicyRuleArgs{
						ApiGroups: pulumi.StringArray{
							pulumi.String("apps"),
						},
						Resources: pulumi.StringArray{
							pulumi.String("replicasets"),
							pulumi.String("deployments"),
						},
						Verbs: pulumi.StringArray{
							pulumi.String("get"),
						},
					},
					&rbacv1.PolicyRuleArgs{
						ApiGroups: pulumi.StringArray{
							pulumi.String("pulumi.com"),
						},
						Resources: pulumi.StringArray{
							pulumi.String("*"),
						},
						Verbs: pulumi.StringArray{
							pulumi.String("create"),
							pulumi.String("delete"),
							pulumi.String("get"),
							pulumi.String("list"),
							pulumi.String("patch"),
							pulumi.String("update"),
							pulumi.String("watch"),
						},
					},
					&rbacv1.PolicyRuleArgs{
						ApiGroups: pulumi.StringArray{
							pulumi.String("coordination.k8s.io"),
						},
						Resources: pulumi.StringArray{
							pulumi.String("leases"),
						},
						Verbs: pulumi.StringArray{
							pulumi.String("create"),
							pulumi.String("get"),
							pulumi.String("list"),
							pulumi.String("update"),
						},
					},
				},
			})
			if err != nil {
				return err
			}

			_, err = rbacv1.NewRoleBinding(ctx, fmt.Sprintf("operator-role-binding-%s", ns), &rbacv1.RoleBindingArgs{
				Metadata: commonObjectMeta,
				Subjects: rbacv1.SubjectArray{
					&rbacv1.SubjectArgs{
						Kind: pulumi.String("ServiceAccount"),
						Name: operatorServiceAccount.Metadata.Name().Elem(),
					},
				},
				RoleRef: &rbacv1.RoleRefArgs{
					Kind:     pulumi.String("Role"),
					Name:     operatorRole.Metadata.Name().Elem(),
					ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
				},
			})
			if err != nil {
				return err
			}

			_, err = appsv1.NewDeployment(ctx, fmt.Sprintf("pulumi-kubernetes-operator-%s", ns), &appsv1.DeploymentArgs{
				Metadata: commonObjectMeta,
				Spec: &appsv1.DeploymentSpecArgs{
					Replicas: pulumi.Int(1),
					Selector: &metav1.LabelSelectorArgs{
						MatchLabels: pulumi.StringMap{
							"name": pulumi.String("pulumi-kubernetes-operator"),
						},
					},
					Template: &corev1.PodTemplateSpecArgs{
						Metadata: &metav1.ObjectMetaArgs{
							Labels: pulumi.StringMap{
								"name": pulumi.String("pulumi-kubernetes-operator"),
							},
						},
						Spec: &corev1.PodSpecArgs{
							ServiceAccountName: operatorServiceAccount.Metadata.Name(),
							Containers: corev1.ContainerArray{
								&corev1.ContainerArgs{
									Name:  pulumi.String("pulumi-kubernetes-operator"),
									Image: pulumi.String(fmt.Sprintf("pulumi/pulumi-kubernetes-operator:%s", operatorVersion)),
									Command: pulumi.StringArray{
										pulumi.String("pulumi-kubernetes-operator"),
									},
									Args: pulumi.StringArray{
										pulumi.String("--zap-level=error"),
										pulumi.String("--zap-time-encoding=iso8601"),
									},
									ImagePullPolicy: pulumi.String("Always"),
									Env: corev1.EnvVarArray{
										&corev1.EnvVarArgs{
											Name: pulumi.String("WATCH_NAMESPACE"),
											ValueFrom: &corev1.EnvVarSourceArgs{
												FieldRef: &corev1.ObjectFieldSelectorArgs{
													FieldPath: pulumi.String("metadata.namespace"),
												},
											},
										},
										&corev1.EnvVarArgs{
											Name: pulumi.String("POD_NAME"),
											ValueFrom: &corev1.EnvVarSourceArgs{
												FieldRef: &corev1.ObjectFieldSelectorArgs{
													FieldPath: pulumi.String("metadata.name"),
												},
											},
										},
										&corev1.EnvVarArgs{
											Name:  pulumi.String("OPERATOR_NAME"),
											Value: pulumi.String("pulumi-kubernetes-operator"),
										},
										&corev1.EnvVarArgs{
											Name:  pulumi.String("GRACEFUL_SHUTDOWN_TIMEOUT_DURATION"),
											Value: pulumi.String("5m"),
										},
										&corev1.EnvVarArgs{
											Name:  pulumi.String("MAX_CONCURRENT_RECONCILES"),
											Value: pulumi.String("10"),
										},
									},
								},
							},
							// Should be same or larger than GRACEFUL_SHUTDOWN_TIMEOUT_DURATION
							TerminationGracePeriodSeconds: pulumi.Int(300),
						},
					},
				},
			}, deploymentOpts...)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
