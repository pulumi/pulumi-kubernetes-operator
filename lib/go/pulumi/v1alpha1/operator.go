package v1alpha1

import (
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v2/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v2/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v2/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v2/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v2/go/kubernetes/yaml"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

type Operator struct {
	pulumi.ResourceState
}

func NewOperator(ctx *pulumi.Context, name string, namespace *corev1.Namespace, opts ...pulumi.ResourceOption) (*Operator, error) {

	operator := &Operator{}
	err := ctx.RegisterComponentResource("pulumi:v1alpha1:Operator", name, operator, opts...)
	if err != nil {
		return nil, err
	}

	pulumiCRD, err := yaml.NewConfigFile(ctx, "pulumi-stack-crd", &yaml.ConfigFileArgs{
		File: "./deploy/crds/pulumi.com_stacks.yaml",
	},
		pulumi.Parent(operator),
	)
	if err != nil {
		return nil, err
	}

	serviceAccount, err := corev1.NewServiceAccount(ctx, "operator-serviceaccount", &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Namespace: namespace.Metadata.Name().Elem(),
		},
	},
		pulumi.Parent(operator),
	)
	if err != nil {
		return nil, err
	}

	role, err := rbacv1.NewRole(ctx, "operator-role", &rbacv1.RoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Namespace: namespace.Metadata.Name().Elem(),
		},
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
		},
	},
		pulumi.Parent(operator),
	)
	if err != nil {
		return nil, err
	}

	_, err = rbacv1.NewRoleBinding(ctx, "operator-role-binding", &rbacv1.RoleBindingArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Namespace: namespace.Metadata.Name().Elem(),
		},
		Subjects: rbacv1.SubjectArray{
			&rbacv1.SubjectArgs{
				Kind: pulumi.String("ServiceAccount"),
				Name: serviceAccount.Metadata.Name().Elem(),
			},
		},
		RoleRef: &rbacv1.RoleRefArgs{
			Kind:     pulumi.String("Role"),
			Name:     role.Metadata.Name().Elem(),
			ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
		},
	},
		pulumi.Parent(operator),
	)
	if err != nil {
		return nil, err
	}

	_, err = appsv1.NewDeployment(ctx, "operator", &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Namespace: namespace.Metadata.Name().Elem(),
		},
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
					ServiceAccountName: serviceAccount.Metadata.Name().Elem(),
					ImagePullSecrets: corev1.LocalObjectReferenceArray{
						&corev1.LocalObjectReferenceArgs{
							Name: pulumi.String("pulumi-kubernetes-operator"),
						},
					},
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Image: pulumi.String("pulumi/pulumi-kubernetes-operator:v0.0.7"),
							Command: pulumi.StringArray{
								pulumi.String("pulumi-kubernetes-operator"),
							},
							Name: pulumi.String("pulumi-kubernetes-operator"),
							Args: pulumi.StringArray{
								pulumi.String("--zap-level=debug"),
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
							},
							Resources: corev1.ResourceRequirementsArgs{
								Limits: pulumi.StringMap{
									"cpu":    pulumi.String("50m"),
									"memory": pulumi.String("32Mi"),
								},
								Requests: pulumi.StringMap{
									"cpu":    pulumi.String("50m"),
									"memory": pulumi.String("32Mi"),
								},
							},
						},
					},
				},
			},
		},
	},
		pulumi.DependsOn([]pulumi.Resource{pulumiCRD}),
		pulumi.Parent(operator),
	)
	if err != nil {
		return nil, err
	}
	return operator, nil
}