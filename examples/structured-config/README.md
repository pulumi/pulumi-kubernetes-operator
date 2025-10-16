# Structured Configuration Examples

This directory contains examples demonstrating the structured configuration feature in Pulumi Kubernetes Operator v2.3.0+.

## Overview

Prior to v2.3.0, the Pulumi Kubernetes Operator only supported string configuration values. Starting with v2.3.0, you can now use **structured configuration values** including:

- **Objects/Maps**: Complex nested structures
- **Arrays/Lists**: Ordered collections of values
- **Numbers**: Integers and floating-point values
- **Booleans**: `true` or `false`
- **Strings**: String values

## Requirements

- **Pulumi Kubernetes Operator**: v2.3.0 or later
- **Pulumi CLI**: v3.202.0 or later in your workspace image

## Examples

### 1. Inline Structured Configuration

**File**: [`inline-config.yaml`](./inline-config.yaml)

Demonstrates using structured values directly in the Stack spec:

```yaml
apiVersion: pulumi.com/v1
kind: Stack
spec:
  config:
    # String values (traditional)
    aws:region: us-west-2

    # Object value (NEW!)
    databaseConfig:
      host: db.example.com
      port: 5432
      ssl: true

    # Array value (NEW!)
    allowedRegions:
      - us-west-2
      - us-east-1

    # Number value (NEW!)
    maxRetries: 3

    # Boolean value (NEW!)
    enableCaching: true
```

**Use case**: Quick configuration directly in the Stack manifest, ideal for development environments.

### 2. ConfigMap-based Configuration

**File**: [`configmap-config.yaml`](./configmap-config.yaml)

Demonstrates loading structured configuration from Kubernetes ConfigMaps:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  settings.json: |
    {
      "apiEndpoint": "https://api.example.com",
      "features": {
        "authentication": true,
        "rateLimit": 1000
      }
    }
---
apiVersion: pulumi.com/v1
kind: Stack
spec:
  # Reference the ConfigMap with JSON parsing
  configRef:
    appSettings:
      name: app-config
      key: settings.json
      json: true  # Parse as JSON object
```

**Use case**: Separate configuration management, GitOps workflows, or shared config across multiple stacks.


## Key Features

### Replace Semantics

Structured values use **replace semantics** - setting a config key replaces its entire value:

```yaml
# Initial configuration
config:
  database:
    host: localhost
    port: 5432

# Later update - this REPLACES the entire 'database' object
config:
  database:
    host: prod.example.com
    # port is now MISSING - not merged!
```

### JSON Flag for References

When loading from ConfigMaps, use `json: true` to parse as structured data:

```yaml
configRef:
  settings:
    name: my-configmap
    key: config.json
    json: true  # Parses the value as JSON

  textValue:
    name: my-configmap
    key: plain.txt
    # No json flag - treated as plain string
```

## Limitations

### 1. No Nested Path Updates

You cannot update a property within an object - you must replace the whole object:

```yaml
# ❌ NOT SUPPORTED: Update just the port
config:
  database.port: 3306  # Won't work

# ✅ SUPPORTED: Replace entire object
config:
  database:
    host: localhost
    port: 3306
```

### 2. Secrets Must Be Strings

The `secretsRef` field **only supports string values**, not structured data:

```yaml
# ❌ NOT SUPPORTED: Structured secret
secretsRef:
  dbCreds:
    type: Secret
    secret:
      name: db-creds
      key: config.json
      json: true  # Not supported for secrets

# ✅ SUPPORTED: String secret
secretsRef:
  apiKey:
    type: Secret
    secret:
      name: api-secrets
      key: token  # String value only
```

### 3. Version Requirements

If you use structured configuration with a Pulumi CLI version older than v3.202.0, the Stack will enter a **Stalled** state:

```yaml
status:
  conditions:
    - type: Stalled
      status: "True"
      reason: PulumiVersionTooLow
      message: "Pulumi CLI version v3.202.0 or higher required for JSON configuration support"
```

**Solution**: Update your workspace image to `pulumi/pulumi:3.202.0-nonroot` or later.

## Related Documentation

- [Pulumi Configuration](https://www.pulumi.com/docs/intro/concepts/config/)
- [Pulumi Kubernetes Operator](https://www.pulumi.com/docs/using-pulumi/continuous-delivery/pulumi-kubernetes-operator/)
- [Stack API Reference](../../docs/stacks.md)
- [Workspace API Reference](../../docs/workspaces.md)

## Questions or Issues?

- [File an issue](https://github.com/pulumi/pulumi-kubernetes-operator/issues)
- [Join the Pulumi Community Slack](https://slack.pulumi.com/)
