// Copyright 2021, Pulumi Corporation.  All rights reserved.

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
