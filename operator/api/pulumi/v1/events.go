/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

// StackEvent is a manifestation of a Kubernetes event emitted by the stack controller.
type StackEvent struct {
	reason    StackEventReason
	eventType StackEventType
}

func (e StackEvent) EventType() string {
	return string(e.eventType)
}

func (e StackEvent) Reason() string {
	return string(e.reason)
}

// StackEventType tracks the types supported by the Kubernetes EventRecorder interface in k8s.io/client-go/tools/record
type StackEventType string

const (
	EventTypeNormal  StackEventType = "Normal"
	EventTypeWarning StackEventType = "Warning"
)

// StackEventReason reflects distinct categorizations of events emitted by the stack controller.
type StackEventReason string

const (
	// Warnings

	StackConfigInvalid          StackEventReason = "StackConfigInvalid"
	StackInitializationFailure  StackEventReason = "StackInitializationFailure"
	StackGitAuthFailure         StackEventReason = "StackGitAuthenticationFailure"
	StackUpdateFailure          StackEventReason = "StackUpdateFailure"
	StackUpdateConflictDetected StackEventReason = "StackUpdateConflictDetected"
	StackOutputRetrievalFailure StackEventReason = "StackOutputRetrievalFailure"

	// Normals

	StackUpdateDetected   StackEventReason = "StackUpdateDetected"
	StackNotFound         StackEventReason = "StackNotFound"
	StackUpdateSuccessful StackEventReason = "StackCreated"
)

func StackConfigInvalidEvent() StackEvent {
	return StackEvent{eventType: EventTypeWarning, reason: StackConfigInvalid}
}

func StackInitializationFailureEvent() StackEvent {
	return StackEvent{eventType: EventTypeWarning, reason: StackInitializationFailure}
}

func StackGitAuthFailureEvent() StackEvent {
	return StackEvent{eventType: EventTypeWarning, reason: StackGitAuthFailure}
}

func StackUpdateFailureEvent() StackEvent {
	return StackEvent{eventType: EventTypeWarning, reason: StackUpdateFailure}
}

func StackUpdateConflictDetectedEvent() StackEvent {
	return StackEvent{eventType: EventTypeWarning, reason: StackUpdateConflictDetected}
}

func StackOutputRetrievalFailureEvent() StackEvent {
	return StackEvent{eventType: EventTypeWarning, reason: StackOutputRetrievalFailure}
}

func StackUpdateDetectedEvent() StackEvent {
	return StackEvent{eventType: EventTypeNormal, reason: StackUpdateDetected}
}

func StackNotFoundEvent() StackEvent {
	return StackEvent{eventType: EventTypeNormal, reason: StackNotFound}
}

func StackUpdateSuccessfulEvent() StackEvent {
	return StackEvent{eventType: EventTypeNormal, reason: StackUpdateSuccessful}
}
