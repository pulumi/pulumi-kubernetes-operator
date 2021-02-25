using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

class Operator : Pulumi.ComponentResource
{
    public Operator(string name, ComponentResourceOptions opts)
        : base("pulumi:v1alpha1:Operator", name, opts)
    {

        var operatorServiceAccount = new Kubernetes.Core.v1.ServiceAccount("operatorServiceAccount", new Kubernetes.Core.v1.ServiceAccountArgs
        {
            Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
            {
                Name = "pulumi-kubernetes-operator",
            },
        }, new CustomResourceOptions { Parent= this});

        var operatorRole = new Kubernetes.Rbac.v1.Role("operatorRole", new Kubernetes.Rbac.v1.RoleArgs
        {
            Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
            {
                Name = "pulumi-kubernetes-operator",
            },
            Rules =
            {
                new Kubernetes.Rbac.Inputs.PolicyRuleArgs
                {
                    ApiGroups =
                    {
                        "",
                    },
                    Resources =
                    {
                        "pods",
                        "services",
                        "services/finalizers",
                        "endpoints",
                        "persistentvolumeclaims",
                        "events",
                        "configmaps",
                        "secrets",
                    },
                    Verbs =
                    {
                        "create",
                        "delete",
                        "get",
                        "list",
                        "patch",
                        "update",
                        "watch",
                    },
                },
                new Kubernetes.Rbac.Inputs.PolicyRuleArgs
                {
                    ApiGroups =
                    {
                        "apps",
                    },
                    Resources =
                    {
                        "deployments",
                        "daemonsets",
                        "replicasets",
                        "statefulsets",
                    },
                    Verbs =
                    {
                        "create",
                        "delete",
                        "get",
                        "list",
                        "patch",
                        "update",
                        "watch",
                    },
                },
                new Kubernetes.Rbac.Inputs.PolicyRuleArgs
                {
                    ApiGroups =
                    {
                        "monitoring.coreos.com",
                    },
                    Resources =
                    {
                        "servicemonitors",
                    },
                    Verbs =
                    {
                        "create",
                        "get",
                    },
                },
                new Kubernetes.Rbac.Inputs.PolicyRuleArgs
                {
                    ApiGroups =
                    {
                        "apps",
                    },
                    ResourceNames =
                    {
                        "pulumi-kubernetes-operator",
                    },
                    Resources =
                    {
                        "deployments/finalizers",
                    },
                    Verbs =
                    {
                        "update",
                    },
                },
                new Kubernetes.Rbac.Inputs.PolicyRuleArgs
                {
                    ApiGroups =
                    {
                        "",
                    },
                    Resources =
                    {
                        "pods",
                    },
                    Verbs =
                    {
                        "get",
                    },
                },
                new Kubernetes.Rbac.Inputs.PolicyRuleArgs
                {
                    ApiGroups =
                    {
                        "apps",
                    },
                    Resources =
                    {
                        "replicasets",
                        "deployments",
                    },
                    Verbs =
                    {
                        "get",
                    },
                },
                new Kubernetes.Rbac.Inputs.PolicyRuleArgs
                {
                    ApiGroups =
                    {
                        "pulumi.com",
                    },
                    Resources =
                    {
                        "*",
                    },
                    Verbs =
                    {
                        "create",
                        "delete",
                        "get",
                        "list",
                        "patch",
                        "update",
                        "watch",
                    },
                },
            },
        }, new CustomResourceOptions { Parent = this });

        var operatorRoleBinding = new Kubernetes.Rbac.v1.RoleBinding("operatorRoleBinding", new Kubernetes.Rbac.v1.RoleBindingArgs
        {
            Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
            {
                Name = "pulumi-kubernetes-operator",
            },
            Subjects =
            {
                new Kubernetes.Rbac.Inputs.SubjectArgs
                {
                    Kind = "ServiceAccount",
                    Name = "pulumi-kubernetes-operator",
                },
            },
            RoleRef = new Kubernetes.Rbac.Inputs.RoleRefArgs
            {
                Kind = "Role",
                Name = "pulumi-kubernetes-operator",
                ApiGroup = "rbac.authorization.k8s.io",
            },
        }, new CustomResourceOptions { Parent = this });

        var operatorDeployment = new Kubernetes.Apps.v1.Deployment("operatorDeployment", new Kubernetes.Apps.v1.DeploymentArgs
        {
            Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
            {
                Name = "pulumi-kubernetes-operator",
            },
            Spec = new Kubernetes.Apps.Inputs.DeploymentSpecArgs
            {
                Replicas = 1,
                Selector = new Kubernetes.Meta.Inputs.LabelSelectorArgs
                {
                    MatchLabels =
                    {
                        { "name", "pulumi-kubernetes-operator" },
                    },
                },
                Template = new Kubernetes.Core.Inputs.PodTemplateSpecArgs
                {
                    Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
                    {
                        Labels =
                        {
                            { "name", "pulumi-kubernetes-operator" },
                        },
                    },
                    Spec = new Kubernetes.Core.Inputs.PodSpecArgs
                    {
                        ServiceAccountName = "pulumi-kubernetes-operator",
                        ImagePullSecrets =
                        {
                            new Kubernetes.Core.Inputs.LocalObjectReferenceArgs
                            {
                                Name = "pulumi-kubernetes-operator",
                            },
                        },
                        Containers =
                        {
                            new Kubernetes.Core.Inputs.ContainerArgs
                            {
                                Name = "pulumi-kubernetes-operator",
                                Image = "pulumi/pulumi-kubernetes-operator:v0.0.7",
                                Command =
                                {
                                    "pulumi-kubernetes-operator",
                                },
                                Args =
                                {
                                    "--zap-level=debug",
                                },
                                ImagePullPolicy = "Always",
                                Env =
                                {
                                    new Kubernetes.Core.Inputs.EnvVarArgs
                                    {
                                        Name = "WATCH_NAMESPACE",
                                        ValueFrom = new Kubernetes.Core.Inputs.EnvVarSourceArgs
                                        {
                                            FieldRef = new Kubernetes.Core.Inputs.ObjectFieldSelectorArgs
                                            {
                                                FieldPath = "metadata.namespace",
                                            },
                                        },
                                    },
                                    new Kubernetes.Core.Inputs.EnvVarArgs
                                    {
                                        Name = "POD_NAME",
                                        ValueFrom = new Kubernetes.Core.Inputs.EnvVarSourceArgs
                                        {
                                            FieldRef = new Kubernetes.Core.Inputs.ObjectFieldSelectorArgs
                                            {
                                                FieldPath = "metadata.name",
                                            },
                                        },
                                    },
                                    new Kubernetes.Core.Inputs.EnvVarArgs
                                    {
                                        Name = "OPERATOR_NAME",
                                        Value = "pulumi-kubernetes-operator",
                                    },
                                },
                            },
                        },
                    },
                },
            },
        }, new CustomResourceOptions { Parent = this });

        this.RegisterOutputs();
    }
}
