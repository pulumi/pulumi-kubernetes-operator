# API Reference

Packages:

- [auto.pulumi.com/v1alpha1](#autopulumicomv1alpha1)

# auto.pulumi.com/v1alpha1

Resource Types:

- [Update](#update)




## Update
<sup><sup>[↩ Parent](#autopulumicomv1alpha1 )</sup></sup>






Update is the Schema for the updates API

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
      <td>auto.pulumi.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Update</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#updatespec">spec</a></b></td>
        <td>object</td>
        <td>
          UpdateSpec defines the desired state of Update<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#updatestatus">status</a></b></td>
        <td>object</td>
        <td>
          UpdateStatus defines the observed state of Update<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Update.spec
<sup><sup>[↩ Parent](#update)</sup></sup>



UpdateSpec defines the desired state of Update

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
        <td><b>continueOnError</b></td>
        <td>boolean</td>
        <td>
          ContinueOnError will continue to perform the update operation despite the
occurrence of errors.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>expectNoChanges</b></td>
        <td>boolean</td>
        <td>
          Return an error if any changes occur during this update<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          Message (optional) to associate with the preview operation<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>parallel</b></td>
        <td>integer</td>
        <td>
          Parallel is the number of resource operations to run in parallel at once
(1 for no parallelism). Defaults to unbounded.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>refresh</b></td>
        <td>boolean</td>
        <td>
          refresh will run a refresh before the update.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>remove</b></td>
        <td>boolean</td>
        <td>
          Remove the stack and its configuration after all resources in the stack
have been deleted.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>replace</b></td>
        <td>[]string</td>
        <td>
          Specify resources to replace<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>stackName</b></td>
        <td>string</td>
        <td>
          Specify the Pulumi stack to select for the update.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>target</b></td>
        <td>[]string</td>
        <td>
          Specify an exclusive list of resource URNs to update<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>targetDependents</b></td>
        <td>boolean</td>
        <td>
          TargetDependents allows updating of dependent targets discovered but not
specified in the Target list<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>ttlAfterCompleted</b></td>
        <td>string</td>
        <td>
          TTL for a completed update object.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          Type of the update to perform.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>workspaceName</b></td>
        <td>string</td>
        <td>
          WorkspaceName is the workspace to update.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Update.status
<sup><sup>[↩ Parent](#update)</sup></sup>



UpdateStatus defines the observed state of Update

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
        <td><b><a href="#updatestatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Represents the observations of an update's current state.
Known .status.conditions.type are: "Complete", "Failed", and "Progressing"<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>endTime</b></td>
        <td>string</td>
        <td>
          The end time of the operation.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the status was set based upon.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>outputs</b></td>
        <td>string</td>
        <td>
          Outputs names a secret containing the outputs for this update.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>permalink</b></td>
        <td>string</td>
        <td>
          Represents the permalink URL in the Pulumi Console for the operation. Not available for DIY backends.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>startTime</b></td>
        <td>string</td>
        <td>
          The start time of the operation.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Update.status.conditions[index]
<sup><sup>[↩ Parent](#updatestatus)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.

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
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>