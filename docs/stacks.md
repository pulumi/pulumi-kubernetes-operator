# API Reference

Packages:

- [pulumi.com/v1](#pulumicomv1)
- [pulumi.com/v1alpha1](#pulumicomv1alpha1)

# pulumi.com/v1

Resource Types:

- [Stack](#stack)




## Stack
<sup><sup>[↩ Parent](#pulumicomv1 )</sup></sup>






Stack is the Schema for the stacks API

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
      <td>Stack</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspec">spec</a></b></td>
        <td>object</td>
        <td>
          StackSpec defines the desired state of Pulumi Stack being managed by this operator.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackstatus">status</a></b></td>
        <td>object</td>
        <td>
          StackStatus defines the observed state of Stack<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec
<sup><sup>[↩ Parent](#stack)</sup></sup>



StackSpec defines the desired state of Pulumi Stack being managed by this operator.

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
        <td><b>stack</b></td>
        <td>string</td>
        <td>
          Stack is the fully qualified name of the stack to deploy (<org>/<stack>).<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>accessTokenSecret</b></td>
        <td>string</td>
        <td>
          (optional) AccessTokenSecret is the name of a Secret containing the PULUMI_ACCESS_TOKEN for Pulumi access.
Deprecated: use EnvRefs with a "secret" entry with the key PULUMI_ACCESS_TOKEN instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>backend</b></td>
        <td>string</td>
        <td>
          (optional) Backend is an optional backend URL to use for all Pulumi operations.<br/>
Examples:<br/>
  - Pulumi Service:              "https://app.pulumi.com" (default)<br/>
  - Self-managed Pulumi Service: "https://pulumi.acmecorp.com" <br/>
  - Local:                       "file://./einstein" <br/>
  - AWS:                         "s3://<my-pulumi-state-bucket>" <br/>
  - Azure:                       "azblob://<my-pulumi-state-bucket>" <br/>
  - GCP:                         "gs://<my-pulumi-state-bucket>" <br/>
See: https://www.pulumi.com/docs/intro/concepts/state/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>branch</b></td>
        <td>string</td>
        <td>
          (optional) Branch is the branch name to deploy, either the simple or fully qualified ref name, e.g. refs/heads/master. This
is mutually exclusive with the Commit setting. Either value needs to be specified.
When specified, the operator will periodically poll to check if the branch has any new commits.
The frequency of the polling is configurable through ResyncFrequencySeconds, defaulting to every 60 seconds.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>commit</b></td>
        <td>string</td>
        <td>
          (optional) Commit is the hash of the commit to deploy. If used, HEAD will be in detached mode. This
is mutually exclusive with the Branch setting. Either value needs to be specified.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>config</b></td>
        <td>map[string]string</td>
        <td>
          (optional) Config is the configuration for this stack, which can be optionally specified inline. If this
is omitted, configuration is assumed to be checked in and taken from the source repository.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>continueResyncOnCommitMatch</b></td>
        <td>boolean</td>
        <td>
          (optional) ContinueResyncOnCommitMatch - when true - informs the operator to continue trying
to update stacks even if the revision of the source matches. This might be useful in
environments where Pulumi programs have dynamic elements for example, calls to internal APIs
where GitOps style commit tracking is not sufficient.  Defaults to false, i.e. when a
particular revision is successfully run, the operator will not attempt to rerun the program
at that revision again.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>destroyOnFinalize</b></td>
        <td>boolean</td>
        <td>
          (optional) DestroyOnFinalize can be set to true to destroy the stack completely upon deletion of the Stack custom resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecenvrefskey">envRefs</a></b></td>
        <td>map[string]object</td>
        <td>
          (optional) EnvRefs is an optional map containing environment variables as keys and stores descriptors to where
the variables' values should be loaded from (one of literal, environment variable, file on the
filesystem, or Kubernetes Secret) as values.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>envSecrets</b></td>
        <td>[]string</td>
        <td>
          (optional) SecretEnvs is an optional array of Secret names containing environment variables to set.
Deprecated: use EnvRefs instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>envs</b></td>
        <td>[]string</td>
        <td>
          (optional) Envs is an optional array of config maps containing environment variables to set.
Deprecated: use EnvRefs instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>expectNoRefreshChanges</b></td>
        <td>boolean</td>
        <td>
          (optional) ExpectNoRefreshChanges can be set to true if a stack is not expected to have
changes during a refresh before the update is run.
This could occur, for example, is a resource's state is changing outside of Pulumi
(e.g., metadata, timestamps).<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecfluxsource">fluxSource</a></b></td>
        <td>object</td>
        <td>
          FluxSource specifies how to fetch source code from a Flux source object.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauth">gitAuth</a></b></td>
        <td>object</td>
        <td>
          (optional) GitAuth allows configuring git authentication options
There are 3 different authentication options:
  * SSH private key (and its optional password)
  * Personal access token
  * Basic auth username and password
Only one authentication mode will be considered if more than one option is specified,
with ssh private key/password preferred first, then personal access token, and finally
basic auth credentials.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>gitAuthSecret</b></td>
        <td>string</td>
        <td>
          (optional) GitAuthSecret is the the name of a Secret containing an
authentication option for the git repository.
There are 3 different authentication options:
  * Personal access token
  * SSH private key (and it's optional password)
  * Basic auth username and password
Only one authentication mode will be considered if more than one option is specified,
with ssh private key/password preferred first, then personal access token, and finally
basic auth credentials.
Deprecated. Use GitAuth instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecprerequisitesindex">prerequisites</a></b></td>
        <td>[]object</td>
        <td>
          (optional) Prerequisites is a list of references to other stacks, each with a constraint on
how long ago it must have succeeded. This can be used to make sure e.g., state is
re-evaluated before running a stack that depends on it.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecprogramref">programRef</a></b></td>
        <td>object</td>
        <td>
          ProgramRef refers to a Program object, to be used as the source for the stack.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>projectRepo</b></td>
        <td>string</td>
        <td>
          ProjectRepo is the git source control repository from which we fetch the project code and configuration.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>refresh</b></td>
        <td>boolean</td>
        <td>
          (optional) Refresh can be set to true to refresh the stack before it is updated.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>repoDir</b></td>
        <td>string</td>
        <td>
          (optional) RepoDir is the directory to work from in the project's source repository
where Pulumi.yaml is located. It is used in case Pulumi.yaml is not
in the project source root.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>resyncFrequencySeconds</b></td>
        <td>integer</td>
        <td>
          (optional) ResyncFrequencySeconds when set to a non-zero value, triggers a resync of the stack at
the specified frequency even if no changes to the custom resource are detected.
If branch tracking is enabled (branch is non-empty), commit polling will occur at this frequency.
The minimal resync frequency supported is 60 seconds. The default value for this field is 60 seconds.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>retryOnUpdateConflict</b></td>
        <td>boolean</td>
        <td>
          (optional) RetryOnUpdateConflict issues a stack update retry reconciliation loop
in the event that the update hits a HTTP 409 conflict due to
another update in progress.
This is only recommended if you are sure that the stack updates are
idempotent, and if you are willing to accept retry loops until
all spawned retries succeed. This will also create a more populated,
and randomized activity timeline for the stack in the Pulumi Service.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secrets</b></td>
        <td>map[string]string</td>
        <td>
          (optional) Secrets is the secret configuration for this stack, which can be optionally specified inline. If this
is omitted, secrets configuration is assumed to be checked in and taken from the source repository.
Deprecated: use SecretRefs instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretsProvider</b></td>
        <td>string</td>
        <td>
          (optional) SecretsProvider is used to initialize a Stack with alternative encryption.
Examples:
  - AWS:   "awskms:///arn:aws:kms:us-east-1:111122223333:key/1234abcd-12ab-34bc-56ef-1234567890ab?region=us-east-1"
  - Azure: "azurekeyvault://acmecorpvault.vault.azure.net/keys/mykeyname"
  - GCP:   "gcpkms://projects/MYPROJECT/locations/MYLOCATION/keyRings/MYKEYRING/cryptoKeys/MYKEY"


See: https://www.pulumi.com/docs/intro/concepts/secrets/#initializing-a-stack-with-alternative-encryption<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecsecretsrefkey">secretsRef</a></b></td>
        <td>map[string]object</td>
        <td>
          (optional) SecretRefs is the secret configuration for this stack which can be specified through ResourceRef.
If this is omitted, secrets configuration is assumed to be checked in and taken from the source repository.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>targets</b></td>
        <td>[]string</td>
        <td>
          (optional) Targets is a list of URNs of resources to update exclusively. If supplied, only
resources mentioned will be updated.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>useLocalStackOnly</b></td>
        <td>boolean</td>
        <td>
          (optional) UseLocalStackOnly can be set to true to prevent the operator from
creating stacks that do not exist in the tracking git repo.
The default behavior is to create a stack if it doesn't exist.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.envRefs[key]
<sup><sup>[↩ Parent](#stackspec)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecenvrefskeyenv">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecenvrefskeyfilesystem">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecenvrefskeyliteral">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecenvrefskeysecret">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.envRefs[key].env
<sup><sup>[↩ Parent](#stackspecenvrefskey)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.envRefs[key].filesystem
<sup><sup>[↩ Parent](#stackspecenvrefskey)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.envRefs[key].literal
<sup><sup>[↩ Parent](#stackspecenvrefskey)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.envRefs[key].secret
<sup><sup>[↩ Parent](#stackspecenvrefskey)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.fluxSource
<sup><sup>[↩ Parent](#stackspec)</sup></sup>



FluxSource specifies how to fetch source code from a Flux source object.

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
        <td><b><a href="#stackspecfluxsourcesourceref">sourceRef</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>dir</b></td>
        <td>string</td>
        <td>
          Dir gives the subdirectory containing the Pulumi project (i.e., containing Pulumi.yaml) of
interest, within the fetched source.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.fluxSource.sourceRef
<sup><sup>[↩ Parent](#stackspecfluxsource)</sup></sup>





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
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth
<sup><sup>[↩ Parent](#stackspec)</sup></sup>



(optional) GitAuth allows configuring git authentication options
There are 3 different authentication options:
  * SSH private key (and its optional password)
  * Personal access token
  * Basic auth username and password
Only one authentication mode will be considered if more than one option is specified,
with ssh private key/password preferred first, then personal access token, and finally
basic auth credentials.

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
        <td><b><a href="#stackspecgitauthaccesstoken">accessToken</a></b></td>
        <td>object</td>
        <td>
          ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauth">basicAuth</a></b></td>
        <td>object</td>
        <td>
          BasicAuth configures git authentication through basic auth —
i.e. username and password. Both UserName and Password are required.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauth">sshAuth</a></b></td>
        <td>object</td>
        <td>
          SSHAuth configures ssh-based auth for git authentication.
SSHPrivateKey is required but password is optional.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.accessToken
<sup><sup>[↩ Parent](#stackspecgitauth)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthaccesstokenenv">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthaccesstokenfilesystem">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthaccesstokenliteral">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthaccesstokensecret">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.accessToken.env
<sup><sup>[↩ Parent](#stackspecgitauthaccesstoken)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.accessToken.filesystem
<sup><sup>[↩ Parent](#stackspecgitauthaccesstoken)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.accessToken.literal
<sup><sup>[↩ Parent](#stackspecgitauthaccesstoken)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.accessToken.secret
<sup><sup>[↩ Parent](#stackspecgitauthaccesstoken)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth
<sup><sup>[↩ Parent](#stackspecgitauth)</sup></sup>



BasicAuth configures git authentication through basic auth —
i.e. username and password. Both UserName and Password are required.

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
        <td><b><a href="#stackspecgitauthbasicauthpassword">password</a></b></td>
        <td>object</td>
        <td>
          ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthusername">userName</a></b></td>
        <td>object</td>
        <td>
          ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.password
<sup><sup>[↩ Parent](#stackspecgitauthbasicauth)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthpasswordenv">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthpasswordfilesystem">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthpasswordliteral">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthpasswordsecret">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.password.env
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthpassword)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.password.filesystem
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthpassword)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.password.literal
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthpassword)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.password.secret
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthpassword)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.userName
<sup><sup>[↩ Parent](#stackspecgitauthbasicauth)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthusernameenv">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthusernamefilesystem">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthusernameliteral">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthusernamesecret">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.userName.env
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthusername)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.userName.filesystem
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthusername)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.userName.literal
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthusername)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.userName.secret
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthusername)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth
<sup><sup>[↩ Parent](#stackspecgitauth)</sup></sup>



SSHAuth configures ssh-based auth for git authentication.
SSHPrivateKey is required but password is optional.

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
        <td><b><a href="#stackspecgitauthsshauthsshprivatekey">sshPrivateKey</a></b></td>
        <td>object</td>
        <td>
          ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthpassword">password</a></b></td>
        <td>object</td>
        <td>
          ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.sshPrivateKey
<sup><sup>[↩ Parent](#stackspecgitauthsshauth)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthsshprivatekeyenv">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthsshprivatekeyfilesystem">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthsshprivatekeyliteral">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthsshprivatekeysecret">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.sshPrivateKey.env
<sup><sup>[↩ Parent](#stackspecgitauthsshauthsshprivatekey)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.sshPrivateKey.filesystem
<sup><sup>[↩ Parent](#stackspecgitauthsshauthsshprivatekey)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.sshPrivateKey.literal
<sup><sup>[↩ Parent](#stackspecgitauthsshauthsshprivatekey)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.sshPrivateKey.secret
<sup><sup>[↩ Parent](#stackspecgitauthsshauthsshprivatekey)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.password
<sup><sup>[↩ Parent](#stackspecgitauthsshauth)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthpasswordenv">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthpasswordfilesystem">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthpasswordliteral">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthpasswordsecret">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.password.env
<sup><sup>[↩ Parent](#stackspecgitauthsshauthpassword)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.password.filesystem
<sup><sup>[↩ Parent](#stackspecgitauthsshauthpassword)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.password.literal
<sup><sup>[↩ Parent](#stackspecgitauthsshauthpassword)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.password.secret
<sup><sup>[↩ Parent](#stackspecgitauthsshauthpassword)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.prerequisites[index]
<sup><sup>[↩ Parent](#stackspec)</sup></sup>



PrerequisiteRef refers to another stack, and gives requirements for the prerequisite to be
considered satisfied.

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the Stack resource that is a prerequisite.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecprerequisitesindexrequirement">requirement</a></b></td>
        <td>object</td>
        <td>
          Requirement gives specific requirements for the prerequisite; the base requirement is that
the referenced stack is in a successful state.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.prerequisites[index].requirement
<sup><sup>[↩ Parent](#stackspecprerequisitesindex)</sup></sup>



Requirement gives specific requirements for the prerequisite; the base requirement is that
the referenced stack is in a successful state.

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
        <td><b>succeededWithinDuration</b></td>
        <td>string</td>
        <td>
          SucceededWithinDuration gives a duration within which the prerequisite must have reached a
succeeded state; e.g., "1h" means "the prerequisite must be successful, and have become so in
the last hour". Fields (should there ever be more than one) are not intended to be mutually
exclusive.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.programRef
<sup><sup>[↩ Parent](#stackspec)</sup></sup>



ProgramRef refers to a Program object, to be used as the source for the stack.

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.secretsRef[key]
<sup><sup>[↩ Parent](#stackspec)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecsecretsrefkeyenv">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecsecretsrefkeyfilesystem">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecsecretsrefkeyliteral">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecsecretsrefkeysecret">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.secretsRef[key].env
<sup><sup>[↩ Parent](#stackspecsecretsrefkey)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.secretsRef[key].filesystem
<sup><sup>[↩ Parent](#stackspecsecretsrefkey)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.secretsRef[key].literal
<sup><sup>[↩ Parent](#stackspecsecretsrefkey)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.secretsRef[key].secret
<sup><sup>[↩ Parent](#stackspecsecretsrefkey)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.status
<sup><sup>[↩ Parent](#stack)</sup></sup>



StackStatus defines the observed state of Stack

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
        <td><b><a href="#stackstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackstatuslastupdate">lastUpdate</a></b></td>
        <td>object</td>
        <td>
          LastUpdate contains details of the status of the last update.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration records the value of .meta.generation at the point the controller last processed this object<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedReconcileRequest</b></td>
        <td>string</td>
        <td>
          ObservedReconcileRequest records the value of the annotation named for
`ReconcileRequestAnnotation` when it was last seen.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>outputs</b></td>
        <td>map[string]JSON</td>
        <td>
          Outputs contains the exported stack output variables resulting from a deployment.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.status.conditions[index]
<sup><sup>[↩ Parent](#stackstatus)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.
---
This struct is intended for direct use as an array at the field path .status.conditions.  For example,
type FooStatus struct{
    // Represents the observations of a foo's current state.
    // Known .status.conditions.type are: "Available", "Progressing", and "Degraded"
    // +patchMergeKey=type
    // +patchStrategy=merge
    // +listType=map
    // +listMapKey=type
    Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`


    // other fields
}

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
          type of condition in CamelCase or in foo.example.com/CamelCase.
---
Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be
useful (see .node.status.conditions), the ability to deconflict is important.
The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)<br/>
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


### Stack.status.lastUpdate
<sup><sup>[↩ Parent](#stackstatus)</sup></sup>



LastUpdate contains details of the status of the last update.

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
        <td><b>lastAttemptedCommit</b></td>
        <td>string</td>
        <td>
          Last commit attempted<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastResyncTime</b></td>
        <td>string</td>
        <td>
          LastResyncTime contains a timestamp for the last time a resync of the stack took place.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastSuccessfulCommit</b></td>
        <td>string</td>
        <td>
          Last commit successfully applied<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>permalink</b></td>
        <td>string</td>
        <td>
          Permalink is the Pulumi Console URL of the stack operation.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>string</td>
        <td>
          State is the state of the stack update - one of `succeeded` or `failed`<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

# pulumi.com/v1alpha1

Resource Types:

- [Stack](#stack)




## Stack
<sup><sup>[↩ Parent](#pulumicomv1alpha1 )</sup></sup>






Stack is the Schema for the stacks API.
Deprecated: Note Stacks from pulumi.com/v1alpha1 is deprecated in favor of pulumi.com/v1.
It is completely backward compatible. Users are strongly encouraged to switch to pulumi.com/v1.

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
      <td>pulumi.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Stack</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspec-1">spec</a></b></td>
        <td>object</td>
        <td>
          StackSpec defines the desired state of Pulumi Stack being managed by this operator.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackstatus-1">status</a></b></td>
        <td>object</td>
        <td>
          StackStatus defines the observed state of Stack<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec
<sup><sup>[↩ Parent](#stack-1)</sup></sup>



StackSpec defines the desired state of Pulumi Stack being managed by this operator.

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
        <td><b>stack</b></td>
        <td>string</td>
        <td>
          Stack is the fully qualified name of the stack to deploy (<org>/<stack>).<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>accessTokenSecret</b></td>
        <td>string</td>
        <td>
          (optional) AccessTokenSecret is the name of a Secret containing the PULUMI_ACCESS_TOKEN for Pulumi access.
Deprecated: use EnvRefs with a "secret" entry with the key PULUMI_ACCESS_TOKEN instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>backend</b></td>
        <td>string</td>
        <td>
          (optional) Backend is an optional backend URL to use for all Pulumi operations.<br/>
Examples:<br/>
  - Pulumi Service:              "https://app.pulumi.com" (default)<br/>
  - Self-managed Pulumi Service: "https://pulumi.acmecorp.com" <br/>
  - Local:                       "file://./einstein" <br/>
  - AWS:                         "s3://<my-pulumi-state-bucket>" <br/>
  - Azure:                       "azblob://<my-pulumi-state-bucket>" <br/>
  - GCP:                         "gs://<my-pulumi-state-bucket>" <br/>
See: https://www.pulumi.com/docs/intro/concepts/state/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>branch</b></td>
        <td>string</td>
        <td>
          (optional) Branch is the branch name to deploy, either the simple or fully qualified ref name, e.g. refs/heads/master. This
is mutually exclusive with the Commit setting. Either value needs to be specified.
When specified, the operator will periodically poll to check if the branch has any new commits.
The frequency of the polling is configurable through ResyncFrequencySeconds, defaulting to every 60 seconds.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>commit</b></td>
        <td>string</td>
        <td>
          (optional) Commit is the hash of the commit to deploy. If used, HEAD will be in detached mode. This
is mutually exclusive with the Branch setting. Either value needs to be specified.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>config</b></td>
        <td>map[string]string</td>
        <td>
          (optional) Config is the configuration for this stack, which can be optionally specified inline. If this
is omitted, configuration is assumed to be checked in and taken from the source repository.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>continueResyncOnCommitMatch</b></td>
        <td>boolean</td>
        <td>
          (optional) ContinueResyncOnCommitMatch - when true - informs the operator to continue trying
to update stacks even if the revision of the source matches. This might be useful in
environments where Pulumi programs have dynamic elements for example, calls to internal APIs
where GitOps style commit tracking is not sufficient.  Defaults to false, i.e. when a
particular revision is successfully run, the operator will not attempt to rerun the program
at that revision again.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>destroyOnFinalize</b></td>
        <td>boolean</td>
        <td>
          (optional) DestroyOnFinalize can be set to true to destroy the stack completely upon deletion of the Stack custom resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecenvrefskey-1">envRefs</a></b></td>
        <td>map[string]object</td>
        <td>
          (optional) EnvRefs is an optional map containing environment variables as keys and stores descriptors to where
the variables' values should be loaded from (one of literal, environment variable, file on the
filesystem, or Kubernetes Secret) as values.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>envSecrets</b></td>
        <td>[]string</td>
        <td>
          (optional) SecretEnvs is an optional array of Secret names containing environment variables to set.
Deprecated: use EnvRefs instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>envs</b></td>
        <td>[]string</td>
        <td>
          (optional) Envs is an optional array of config maps containing environment variables to set.
Deprecated: use EnvRefs instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>expectNoRefreshChanges</b></td>
        <td>boolean</td>
        <td>
          (optional) ExpectNoRefreshChanges can be set to true if a stack is not expected to have
changes during a refresh before the update is run.
This could occur, for example, is a resource's state is changing outside of Pulumi
(e.g., metadata, timestamps).<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecfluxsource-1">fluxSource</a></b></td>
        <td>object</td>
        <td>
          FluxSource specifies how to fetch source code from a Flux source object.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauth-1">gitAuth</a></b></td>
        <td>object</td>
        <td>
          (optional) GitAuth allows configuring git authentication options
There are 3 different authentication options:
  * SSH private key (and its optional password)
  * Personal access token
  * Basic auth username and password
Only one authentication mode will be considered if more than one option is specified,
with ssh private key/password preferred first, then personal access token, and finally
basic auth credentials.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>gitAuthSecret</b></td>
        <td>string</td>
        <td>
          (optional) GitAuthSecret is the the name of a Secret containing an
authentication option for the git repository.
There are 3 different authentication options:
  * Personal access token
  * SSH private key (and it's optional password)
  * Basic auth username and password
Only one authentication mode will be considered if more than one option is specified,
with ssh private key/password preferred first, then personal access token, and finally
basic auth credentials.
Deprecated. Use GitAuth instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecprerequisitesindex-1">prerequisites</a></b></td>
        <td>[]object</td>
        <td>
          (optional) Prerequisites is a list of references to other stacks, each with a constraint on
how long ago it must have succeeded. This can be used to make sure e.g., state is
re-evaluated before running a stack that depends on it.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecprogramref-1">programRef</a></b></td>
        <td>object</td>
        <td>
          ProgramRef refers to a Program object, to be used as the source for the stack.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>projectRepo</b></td>
        <td>string</td>
        <td>
          ProjectRepo is the git source control repository from which we fetch the project code and configuration.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>refresh</b></td>
        <td>boolean</td>
        <td>
          (optional) Refresh can be set to true to refresh the stack before it is updated.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>repoDir</b></td>
        <td>string</td>
        <td>
          (optional) RepoDir is the directory to work from in the project's source repository
where Pulumi.yaml is located. It is used in case Pulumi.yaml is not
in the project source root.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>resyncFrequencySeconds</b></td>
        <td>integer</td>
        <td>
          (optional) ResyncFrequencySeconds when set to a non-zero value, triggers a resync of the stack at
the specified frequency even if no changes to the custom resource are detected.
If branch tracking is enabled (branch is non-empty), commit polling will occur at this frequency.
The minimal resync frequency supported is 60 seconds. The default value for this field is 60 seconds.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>retryOnUpdateConflict</b></td>
        <td>boolean</td>
        <td>
          (optional) RetryOnUpdateConflict issues a stack update retry reconciliation loop
in the event that the update hits a HTTP 409 conflict due to
another update in progress.
This is only recommended if you are sure that the stack updates are
idempotent, and if you are willing to accept retry loops until
all spawned retries succeed. This will also create a more populated,
and randomized activity timeline for the stack in the Pulumi Service.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secrets</b></td>
        <td>map[string]string</td>
        <td>
          (optional) Secrets is the secret configuration for this stack, which can be optionally specified inline. If this
is omitted, secrets configuration is assumed to be checked in and taken from the source repository.
Deprecated: use SecretRefs instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretsProvider</b></td>
        <td>string</td>
        <td>
          (optional) SecretsProvider is used to initialize a Stack with alternative encryption.
Examples:
  - AWS:   "awskms:///arn:aws:kms:us-east-1:111122223333:key/1234abcd-12ab-34bc-56ef-1234567890ab?region=us-east-1"
  - Azure: "azurekeyvault://acmecorpvault.vault.azure.net/keys/mykeyname"
  - GCP:   "gcpkms://projects/MYPROJECT/locations/MYLOCATION/keyRings/MYKEYRING/cryptoKeys/MYKEY"


See: https://www.pulumi.com/docs/intro/concepts/secrets/#initializing-a-stack-with-alternative-encryption<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecsecretsrefkey-1">secretsRef</a></b></td>
        <td>map[string]object</td>
        <td>
          (optional) SecretRefs is the secret configuration for this stack which can be specified through ResourceRef.
If this is omitted, secrets configuration is assumed to be checked in and taken from the source repository.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>targets</b></td>
        <td>[]string</td>
        <td>
          (optional) Targets is a list of URNs of resources to update exclusively. If supplied, only
resources mentioned will be updated.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>useLocalStackOnly</b></td>
        <td>boolean</td>
        <td>
          (optional) UseLocalStackOnly can be set to true to prevent the operator from
creating stacks that do not exist in the tracking git repo.
The default behavior is to create a stack if it doesn't exist.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.envRefs[key]
<sup><sup>[↩ Parent](#stackspec-1)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecenvrefskeyenv-1">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecenvrefskeyfilesystem-1">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecenvrefskeyliteral-1">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecenvrefskeysecret-1">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.envRefs[key].env
<sup><sup>[↩ Parent](#stackspecenvrefskey-1)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.envRefs[key].filesystem
<sup><sup>[↩ Parent](#stackspecenvrefskey-1)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.envRefs[key].literal
<sup><sup>[↩ Parent](#stackspecenvrefskey-1)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.envRefs[key].secret
<sup><sup>[↩ Parent](#stackspecenvrefskey-1)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.fluxSource
<sup><sup>[↩ Parent](#stackspec-1)</sup></sup>



FluxSource specifies how to fetch source code from a Flux source object.

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
        <td><b><a href="#stackspecfluxsourcesourceref-1">sourceRef</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>dir</b></td>
        <td>string</td>
        <td>
          Dir gives the subdirectory containing the Pulumi project (i.e., containing Pulumi.yaml) of
interest, within the fetched source.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.fluxSource.sourceRef
<sup><sup>[↩ Parent](#stackspecfluxsource-1)</sup></sup>





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
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth
<sup><sup>[↩ Parent](#stackspec-1)</sup></sup>



(optional) GitAuth allows configuring git authentication options
There are 3 different authentication options:
  * SSH private key (and its optional password)
  * Personal access token
  * Basic auth username and password
Only one authentication mode will be considered if more than one option is specified,
with ssh private key/password preferred first, then personal access token, and finally
basic auth credentials.

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
        <td><b><a href="#stackspecgitauthaccesstoken-1">accessToken</a></b></td>
        <td>object</td>
        <td>
          ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauth-1">basicAuth</a></b></td>
        <td>object</td>
        <td>
          BasicAuth configures git authentication through basic auth —
i.e. username and password. Both UserName and Password are required.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauth-1">sshAuth</a></b></td>
        <td>object</td>
        <td>
          SSHAuth configures ssh-based auth for git authentication.
SSHPrivateKey is required but password is optional.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.accessToken
<sup><sup>[↩ Parent](#stackspecgitauth-1)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthaccesstokenenv-1">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthaccesstokenfilesystem-1">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthaccesstokenliteral-1">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthaccesstokensecret-1">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.accessToken.env
<sup><sup>[↩ Parent](#stackspecgitauthaccesstoken-1)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.accessToken.filesystem
<sup><sup>[↩ Parent](#stackspecgitauthaccesstoken-1)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.accessToken.literal
<sup><sup>[↩ Parent](#stackspecgitauthaccesstoken-1)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.accessToken.secret
<sup><sup>[↩ Parent](#stackspecgitauthaccesstoken-1)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth
<sup><sup>[↩ Parent](#stackspecgitauth-1)</sup></sup>



BasicAuth configures git authentication through basic auth —
i.e. username and password. Both UserName and Password are required.

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
        <td><b><a href="#stackspecgitauthbasicauthpassword-1">password</a></b></td>
        <td>object</td>
        <td>
          ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthusername-1">userName</a></b></td>
        <td>object</td>
        <td>
          ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.password
<sup><sup>[↩ Parent](#stackspecgitauthbasicauth-1)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthpasswordenv-1">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthpasswordfilesystem-1">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthpasswordliteral-1">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthpasswordsecret-1">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.password.env
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthpassword-1)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.password.filesystem
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthpassword-1)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.password.literal
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthpassword-1)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.password.secret
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthpassword-1)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.userName
<sup><sup>[↩ Parent](#stackspecgitauthbasicauth-1)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthusernameenv-1">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthusernamefilesystem-1">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthusernameliteral-1">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthbasicauthusernamesecret-1">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.userName.env
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthusername-1)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.userName.filesystem
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthusername-1)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.userName.literal
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthusername-1)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.basicAuth.userName.secret
<sup><sup>[↩ Parent](#stackspecgitauthbasicauthusername-1)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth
<sup><sup>[↩ Parent](#stackspecgitauth-1)</sup></sup>



SSHAuth configures ssh-based auth for git authentication.
SSHPrivateKey is required but password is optional.

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
        <td><b><a href="#stackspecgitauthsshauthsshprivatekey-1">sshPrivateKey</a></b></td>
        <td>object</td>
        <td>
          ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthpassword-1">password</a></b></td>
        <td>object</td>
        <td>
          ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.sshPrivateKey
<sup><sup>[↩ Parent](#stackspecgitauthsshauth-1)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthsshprivatekeyenv-1">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthsshprivatekeyfilesystem-1">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthsshprivatekeyliteral-1">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthsshprivatekeysecret-1">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.sshPrivateKey.env
<sup><sup>[↩ Parent](#stackspecgitauthsshauthsshprivatekey-1)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.sshPrivateKey.filesystem
<sup><sup>[↩ Parent](#stackspecgitauthsshauthsshprivatekey-1)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.sshPrivateKey.literal
<sup><sup>[↩ Parent](#stackspecgitauthsshauthsshprivatekey-1)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.sshPrivateKey.secret
<sup><sup>[↩ Parent](#stackspecgitauthsshauthsshprivatekey-1)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.password
<sup><sup>[↩ Parent](#stackspecgitauthsshauth-1)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthpasswordenv-1">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthpasswordfilesystem-1">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthpasswordliteral-1">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecgitauthsshauthpasswordsecret-1">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.password.env
<sup><sup>[↩ Parent](#stackspecgitauthsshauthpassword-1)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.password.filesystem
<sup><sup>[↩ Parent](#stackspecgitauthsshauthpassword-1)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.password.literal
<sup><sup>[↩ Parent](#stackspecgitauthsshauthpassword-1)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.gitAuth.sshAuth.password.secret
<sup><sup>[↩ Parent](#stackspecgitauthsshauthpassword-1)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.prerequisites[index]
<sup><sup>[↩ Parent](#stackspec-1)</sup></sup>



PrerequisiteRef refers to another stack, and gives requirements for the prerequisite to be
considered satisfied.

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the Stack resource that is a prerequisite.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecprerequisitesindexrequirement-1">requirement</a></b></td>
        <td>object</td>
        <td>
          Requirement gives specific requirements for the prerequisite; the base requirement is that
the referenced stack is in a successful state.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.prerequisites[index].requirement
<sup><sup>[↩ Parent](#stackspecprerequisitesindex-1)</sup></sup>



Requirement gives specific requirements for the prerequisite; the base requirement is that
the referenced stack is in a successful state.

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
        <td><b>succeededWithinDuration</b></td>
        <td>string</td>
        <td>
          SucceededWithinDuration gives a duration within which the prerequisite must have reached a
succeeded state; e.g., "1h" means "the prerequisite must be successful, and have become so in
the last hour". Fields (should there ever be more than one) are not intended to be mutually
exclusive.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.programRef
<sup><sup>[↩ Parent](#stackspec-1)</sup></sup>



ProgramRef refers to a Program object, to be used as the source for the stack.

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.secretsRef[key]
<sup><sup>[↩ Parent](#stackspec-1)</sup></sup>



ResourceRef identifies a resource from which information can be loaded.
Environment variables, files on the filesystem, Kubernetes Secrets and literal
strings are currently supported.

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
          SelectorType is required and signifies the type of selector. Must be one of:
Env, FS, Secret, Literal<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#stackspecsecretsrefkeyenv-1">env</a></b></td>
        <td>object</td>
        <td>
          Env selects an environment variable set on the operator process<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecsecretsrefkeyfilesystem-1">filesystem</a></b></td>
        <td>object</td>
        <td>
          FileSystem selects a file on the operator's file system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecsecretsrefkeyliteral-1">literal</a></b></td>
        <td>object</td>
        <td>
          LiteralRef refers to a literal value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#stackspecsecretsrefkeysecret-1">secret</a></b></td>
        <td>object</td>
        <td>
          SecretRef refers to a Kubernetes Secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.spec.secretsRef[key].env
<sup><sup>[↩ Parent](#stackspecsecretsrefkey-1)</sup></sup>



Env selects an environment variable set on the operator process

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the environment variable<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.secretsRef[key].filesystem
<sup><sup>[↩ Parent](#stackspecsecretsrefkey-1)</sup></sup>



FileSystem selects a file on the operator's file system

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
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path on the filesystem to use to load information from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.secretsRef[key].literal
<sup><sup>[↩ Parent](#stackspecsecretsrefkey-1)</sup></sup>



LiteralRef refers to a literal value

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
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value to load<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Stack.spec.secretsRef[key].secret
<sup><sup>[↩ Parent](#stackspecsecretsrefkey-1)</sup></sup>



SecretRef refers to a Kubernetes Secret

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
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key within the Secret to use.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the Secret<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
unless namespace isolation is disabled in the controller.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.status
<sup><sup>[↩ Parent](#stack-1)</sup></sup>



StackStatus defines the observed state of Stack

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
        <td><b><a href="#stackstatuslastupdate-1">lastUpdate</a></b></td>
        <td>object</td>
        <td>
          LastUpdate contains details of the status of the last update.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>outputs</b></td>
        <td>map[string]JSON</td>
        <td>
          Outputs contains the exported stack output variables resulting from a deployment.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Stack.status.lastUpdate
<sup><sup>[↩ Parent](#stackstatus-1)</sup></sup>



LastUpdate contains details of the status of the last update.

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
        <td><b>lastAttemptedCommit</b></td>
        <td>string</td>
        <td>
          Last commit attempted<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastResyncTime</b></td>
        <td>string</td>
        <td>
          LastResyncTime contains a timestamp for the last time a resync of the stack took place.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastSuccessfulCommit</b></td>
        <td>string</td>
        <td>
          Last commit successfully applied<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>permalink</b></td>
        <td>string</td>
        <td>
          Permalink is the Pulumi Console URL of the stack operation.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>string</td>
        <td>
          State is the state of the stack update - one of `succeeded` or `failed`<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>