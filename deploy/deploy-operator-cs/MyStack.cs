using Pulumi;
using Kubernetes = Pulumi.Kubernetes;
using Pulumi.Kubernetes.Types.Inputs.Core.V1;
using Pulumi.Kubernetes.Types.Inputs.Apps.V1;
using Pulumi.Kubernetes.Types.Inputs.Meta.V1;
using Pulumi.Kubernetes.Types.Inputs.Rbac.V1;

class MyStack : Stack
{
    public const string DefaultCRDVersion = "v1.12.1";
    public const string DefaultOperatorVersion = "v1.12.1";

    public MyStack()
    {
        var config = new Config();
        var crdVersion = config.Get("crd-version") ?? DefaultCRDVersion;
        var operatorVersion = config.Get("operator-version") ?? DefaultOperatorVersion;

        var stackCRD = new Kubernetes.Yaml.ConfigFile("stackcrd", new Kubernetes.Yaml.ConfigFileArgs{
                File = $"https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/{crdVersion}/deploy/crds/pulumi.com_stacks.yaml"
                    });
        var programCRD = new Kubernetes.Yaml.ConfigFile("programcrd", new Kubernetes.Yaml.ConfigFileArgs{
                File = $"https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/{crdVersion}/deploy/crds/pulumi.com_programs.yaml"
                    });

        var deploymentOptions = new Pulumi.CustomResourceOptions{ DependsOn = {stackCRD, programCRD} };

        var deployNamespace = config.Get("namespace") ?? "default";
        var deployNamespaces = config.GetObject<string[]>("namespaces") ?? new string[] { deployNamespace };

        foreach (string ns in deployNamespaces) {
            var operatorServiceAccount = new Kubernetes.Core.V1.ServiceAccount($"operator-service-account-{ns}", new ServiceAccountArgs{
                    Metadata = new ObjectMetaArgs{Namespace = ns}
                });

            var operatorRole = new Kubernetes.Rbac.V1.Role($"operator-role-{ns}", new RoleArgs
            {
                Metadata = new ObjectMetaArgs{Namespace = ns},
                Rules = 
                {
                    new PolicyRuleArgs
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
                    new PolicyRuleArgs
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
                    new PolicyRuleArgs
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
                    new PolicyRuleArgs
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
                    new PolicyRuleArgs
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
                    new PolicyRuleArgs
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
                    new PolicyRuleArgs
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
                    new PolicyRuleArgs
                    {
                        ApiGroups = 
                        {
                            "coordination.k8s.io",
                        },
                        Resources = 
                        {
                            "leases",
                        },
                        Verbs = 
                        {
                            "create",
                            "get",
                            "list",
                            "update",
                        },
                    },
                },
            });

            var operatorRoleBinding = new Kubernetes.Rbac.V1.RoleBinding($"operator-role-binding-{ns}", new RoleBindingArgs
            {
                Metadata = new ObjectMetaArgs{Namespace = ns},
                Subjects = 
                {
                    new SubjectArgs
                    {
                        Kind = "ServiceAccount",
                        Name = operatorServiceAccount.Metadata.Apply(md => md.Name),
                    },
                },
                RoleRef = new RoleRefArgs
                {
                    Kind = "Role",
                    Name = operatorRole.Metadata.Apply(md => md.Name),
                    ApiGroup = "rbac.authorization.k8s.io",
                },
            });
            var operatorDeployment = new Kubernetes.Apps.V1.Deployment($"pulumi-kubernetes-operator-{ns}", new DeploymentArgs
            {
                Metadata = new ObjectMetaArgs{Namespace = ns},
                Spec = new Kubernetes.Types.Inputs.Apps.V1.DeploymentSpecArgs
                {
                    Replicas = 1,
                    Selector = new LabelSelectorArgs
                    {
                        MatchLabels = 
                        {
                            { "name", "pulumi-kubernetes-operator" },
                        },
                    },
                    Template = new PodTemplateSpecArgs
                    {
                        Metadata = new ObjectMetaArgs
                        {
                            Labels = 
                            {
                                { "name", "pulumi-kubernetes-operator" },
                            },
                        },
                        Spec = new PodSpecArgs
                        {
                            ServiceAccountName = operatorServiceAccount.Metadata.Apply(md => md.Name),
                            Containers = 
                            {
                                new ContainerArgs
                                {
                                    Name = "pulumi-kubernetes-operator",
                                    Image = $"pulumi/pulumi-kubernetes-operator:{operatorVersion}",
                                    Command = 
                                    {
                                        "pulumi-kubernetes-operator",
                                    },
                                    Args = 
                                    {
                                        "--zap-level=error",
                                        "--zap-time-encoding=iso8601",
                                    },
                                    ImagePullPolicy = "Always",
                                    Env = 
                                    {
                                        new EnvVarArgs
                                        {
                                            Name = "WATCH_NAMESPACE",
                                            ValueFrom = new EnvVarSourceArgs
                                            {
                                                FieldRef = new ObjectFieldSelectorArgs
                                                {
                                                    FieldPath = "metadata.namespace",
                                                },
                                            },
                                        },
                                        new EnvVarArgs
                                        {
                                            Name = "POD_NAME",
                                            ValueFrom = new EnvVarSourceArgs
                                            {
                                                FieldRef = new ObjectFieldSelectorArgs
                                                {
                                                    FieldPath = "metadata.name",
                                                },
                                            },
                                        },
                                        new EnvVarArgs
                                        {
                                            Name = "OPERATOR_NAME",
                                            Value = "pulumi-kubernetes-operator",
                                        },
                                        new EnvVarArgs
                                        {
                                            Name = "GRACEFUL_SHUTDOWN_TIMEOUT_DURATION",
                                            Value = "5m",
                                        },
                                        new EnvVarArgs
                                        {
                                            Name = "MAX_CONCURRENT_RECONCILES",
                                            Value = "10",
                                        },
                                    },
                                },
                            },
                            // Should be same or larger than GRACEFUL_SHUTDOWN_TIMEOUT_DURATION
                            TerminationGracePeriodSeconds = 300,
                        },
                    },
                },
            }, deploymentOptions);
        }
    }
}
