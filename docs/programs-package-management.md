## Package Management

The `packages` field allows you to specify external packages that should be installed before running your Pulumi program. This is equivalent to the `packages` section in a traditional Pulumi.yaml file and enables you to use parameterized packages, VCS-based components, and other external dependencies.

### Supported Package Types

#### VCS-based Components
Reference packages from version control systems with specific commits, tags, or branches:

```yaml
apiVersion: pulumi.com/v1
kind: Program
metadata:
  name: program-with-vcs-packages
spec:
  program:
    packages:
      # GitHub repository with specific commit
      talos-component: "https://github.com/dirien/pulumi-talos-go-component@0.0.0-x5ed2a9a45c6d287bd2485f4db1e8c052b164a5ef"
      # GitLab repository with tag
      custom-infra: "https://gitlab.com/example/pulumi-infra@v1.2.3"
```

#### Local File Components
Reference local file paths for development and testing:

```yaml
packages:
  local-component: "file://./components/networking"
  shared-utils: "file://../shared/utils"
```

The Program controller publishes only the generated `Pulumi.yaml`, so a Stack that uses `programRef` has no other files in its workspace. A `file://` path only resolves when those files are already present in the workspace, for example with a Stack that uses a Git or Flux source.

#### Parameterized Packages
Reference packages with specific versions from package registries:

```yaml
packages:
  aws-component: "https://registry.example.com/aws-infrastructure@v2.1.0"
  monitoring-stack: "https://packages.company.com/monitoring@^1.5.0"
```
### Complete Example

Here's a complete example showing a Program with multiple package types:

```yaml
apiVersion: pulumi.com/v1
kind: Program
metadata:
  name: multi-package-example
spec:
  program:
    packages:
      # VCS-based component with specific commit
      talos-go-component: "https://github.com/dirien/pulumi-talos-go-component@0.0.0-x5ed2a9a45c6d287bd2485f4db1e8c052b164a5ef"
      # Versioned package
      example-package: "https://example.com/packages/my-component@v1.2.3"
      
    configuration:
      environment:
        type: String
        default: "dev"
    
    resources:
      example-pod:
        type: kubernetes:core/v1:Pod
        properties:
          metadata:
            name: example-pod
          spec:
            containers:
              - name: nginx
                image: nginx:1.21
    
    outputs:
      podName: ${example-pod.metadata.name}
```

### Generated Pulumi.yaml

When the Program controller processes this resource, it generates a Pulumi.yaml file that includes the packages section:

```yaml
name: multi-package-example
runtime: yaml
packages:
  talos-go-component: "https://github.com/dirien/pulumi-talos-go-component@0.0.0-x5ed2a9a45c6d287bd2485f4db1e8c052b164a5ef"
  example-package: "https://example.com/packages/my-component@v1.2.3"
configuration:
  environment:
    type: String
    default: "dev"
resources:
  example-pod:
    type: kubernetes:core/v1:Pod
    properties:
      metadata:
        name: example-pod
      spec:
        containers:
          - name: nginx
            image: nginx:1.21
outputs:
  podName: ${example-pod.metadata.name}
```

### Usage with Stack Resources

Programs with packages work seamlessly with Stack resources:

```yaml
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: stack-using-packages
spec:
  programRef:
    name: multi-package-example
  stack: dev
  config:
    environment: production
```
