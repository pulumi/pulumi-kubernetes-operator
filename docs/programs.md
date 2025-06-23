# API Reference

Packages:

- [pulumi.com/v1](#pulumicomv1)

# pulumi.com/v1

Resource Types:

- [Program](#program)




## Program
<sup><sup>[↩ Parent](#pulumicomv1 )</sup></sup>






Program is the schema for the inline YAML program API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>pulumi.com/v1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Program</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#programprogram">program</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#programstatus">status</a></b></td>
        <td>object</td>
        <td>
          ProgramStatus defines the observed state of Program.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Program.program
<sup><sup>[↩ Parent](#program)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#programprogramconfigurationkey">configuration</a></b></td>
        <td>map[string]object</td>
        <td>
          configuration specifies the Pulumi config inputs to the deployment.
Either type or default is required.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>outputs</b></td>
        <td>map[string]JSON</td>
        <td>
          outputs specifies the Pulumi stack outputs of the program and how they are computed from the resources.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#programprogramresourceskey">resources</a></b></td>
        <td>map[string]object</td>
        <td>
          resources declares the Pulumi resources that will be deployed and managed by the program.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>variables</b></td>
        <td>map[string]JSON</td>
        <td>
          variables specifies intermediate values of the program; the values of variables are
expressions that can be re-used.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Program.program.configuration[key]
<sup><sup>[↩ Parent](#programprogram)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>default</b></td>
        <td>JSON</td>
        <td>
          default is a value of the appropriate type for the template to use if no value is specified.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>enum</td>
        <td>
          type is the (required) data type for the parameter.<br/>
          <br/>
            <i>Enum</i>: String, Number, List<Number>, List<String><br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Program.program.resources[key]
<sup><sup>[↩ Parent](#programprogram)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type is the Pulumi type token for this resource.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#programprogramresourceskeyget">get</a></b></td>
        <td>object</td>
        <td>
          A getter function for the resource. Supplying get is mutually exclusive to properties.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#programprogramresourceskeyoptions">options</a></b></td>
        <td>object</td>
        <td>
          options contains all resource options supported by Pulumi.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>properties</b></td>
        <td>map[string]JSON</td>
        <td>
          properties contains the primary resource-specific keys and values to initialize the resource state.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Program.program.resources[key].get
<sup><sup>[↩ Parent](#programprogramresourceskey)</sup></sup>



A getter function for the resource. Supplying get is mutually exclusive to properties.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          The ID of the resource to import.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>map[string]JSON</td>
        <td>
          state contains the known properties (input & output) of the resource. This assists
the provider in figuring out the correct resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Program.program.resources[key].options
<sup><sup>[↩ Parent](#programprogramresourceskey)</sup></sup>



options contains all resource options supported by Pulumi.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>additionalSecretOutputs</b></td>
        <td>[]string</td>
        <td>
          additionalSecretOutputs specifies properties that must be encrypted as secrets.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>aliases</b></td>
        <td>[]string</td>
        <td>
          aliases specifies names that this resource used to have, so that renaming or refactoring
doesn’t replace it.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#programprogramresourceskeyoptionscustomtimeouts">customTimeouts</a></b></td>
        <td>object</td>
        <td>
          customTimeouts overrides the default retry/timeout behavior for resource provisioning.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>deleteBeforeReplace</b></td>
        <td>boolean</td>
        <td>
          deleteBeforeReplace overrides the default create-before-delete behavior when replacing.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dependsOn</b></td>
        <td>[]JSON</td>
        <td>
          dependsOn adds explicit dependencies in addition to the ones in the dependency graph.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>ignoreChanges</b></td>
        <td>[]string</td>
        <td>
          ignoreChanges declares that changes to certain properties should be ignored when diffing.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>import</b></td>
        <td>string</td>
        <td>
          import adopts an existing resource from your cloud account under the control of Pulumi.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>parent</b></td>
        <td>JSON</td>
        <td>
          parent resource option specifies a parent for a resource. It is used to associate
children with the parents that encapsulate or are responsible for them.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>protect</b></td>
        <td>boolean</td>
        <td>
          protect prevents accidental deletion of a resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>provider</b></td>
        <td>JSON</td>
        <td>
          provider resource option sets a provider for the resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>providers</b></td>
        <td>map[string]JSON</td>
        <td>
          providers resource option sets a map of providers for the resource and its children.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>
          version specifies a provider plugin version that should be used when operating on a resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Program.program.resources[key].options.customTimeouts
<sup><sup>[↩ Parent](#programprogramresourceskeyoptions)</sup></sup>



customTimeouts overrides the default retry/timeout behavior for resource provisioning.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>create</b></td>
        <td>string</td>
        <td>
          create is the custom timeout for create operations.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>delete</b></td>
        <td>string</td>
        <td>
          delete is the custom timeout for delete operations.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>update</b></td>
        <td>string</td>
        <td>
          update is the custom timeout for update operations.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Program.status
<sup><sup>[↩ Parent](#program)</sup></sup>



ProgramStatus defines the observed state of Program.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#programstatusartifact">artifact</a></b></td>
        <td>object</td>
        <td>
          Artifact represents the last successful artifact generated by program reconciliation.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration is the last observed generation of the Program
object.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Program.status.artifact
<sup><sup>[↩ Parent](#programstatus)</sup></sup>



Artifact represents the last successful artifact generated by program reconciliation.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastUpdateTime</b></td>
        <td>string</td>
        <td>
          LastUpdateTime is the timestamp corresponding to the last update of the
Artifact.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path is the relative file path of the Artifact. It can be used to locate
the file in the root of the Artifact storage on the local file system of
the controller managing the Source.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>revision</b></td>
        <td>string</td>
        <td>
          Revision is a human-readable identifier traceable in the origin source
system. It can be a Git commit SHA, Git tag, a Helm chart version, etc.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>url</b></td>
        <td>string</td>
        <td>
          URL is the HTTP address of the Artifact as exposed by the controller
managing the Source. It can be used to retrieve the Artifact for
consumption, e.g. by another controller applying the Artifact contents.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>digest</b></td>
        <td>string</td>
        <td>
          Digest is the digest of the file in the form of '<algorithm>:<checksum>'.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>metadata</b></td>
        <td>map[string]string</td>
        <td>
          Metadata holds upstream information such as OCI annotations.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>size</b></td>
        <td>integer</td>
        <td>
          Size is the number of bytes in the file.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>