// Copyright 2016-2020, Pulumi Corporation.
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
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/v1"
)

// ProjectFile adds required Pulumi 'project' fields to the Program spec, making it valid to be given to Pulumi.
type ProjectFile struct {
	Name    string `json:"name"`
	Runtime string `json:"runtime"`
	pulumiv1.ProgramSpec
}

// func (sess *StackReconcilerSession) SetupWorkspaceFromYAML(ctx context.Context, programRef shared.ProgramReference) (string, error) {
// 	homeDir := sess.getPulumiHome()
// 	workspaceDir := sess.getWorkspaceDir()
// 	sess.logger.Debug("Setting up pulumi workspace for stack", "stack", sess.stack, "workspace", workspaceDir)

// 	// Create a new workspace.
// 	secretsProvider := auto.SecretsProvider(sess.stack.SecretsProvider)

// 	program := pulumiv1.Program{}
// 	programKey := client.ObjectKey{
// 		Name:      programRef.Name,
// 		Namespace: sess.namespace,
// 	}

// 	err := sess.kubeClient.Get(ctx, programKey, &program)
// 	if err != nil {
// 		return "", errProgramNotFound
// 	}

// 	var project ProjectFile
// 	project.Name = program.Name
// 	project.Runtime = "yaml"
// 	project.ProgramSpec = program.Program

// 	out, err := yaml.Marshal(&project)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to marshal program object to YAML: %w", err)
// 	}

// 	err = os.WriteFile(filepath.Join(workspaceDir, "Pulumi.yaml"), out, 0600)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to write YAML to file: %w", err)
// 	}

// 	var w auto.Workspace
// 	w, err = auto.NewLocalWorkspace(
// 		ctx,
// 		auto.PulumiHome(homeDir),
// 		auto.WorkDir(workspaceDir),
// 		secretsProvider)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create local workspace: %w", err)
// 	}

// 	revision := fmt.Sprintf("%s/%d", program.Name, program.ObjectMeta.Generation)

// 	return revision, sess.setupWorkspace(ctx, w)
// }
