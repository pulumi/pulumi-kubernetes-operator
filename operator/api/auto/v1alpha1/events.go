// Copyright 2016-2025, Pulumi Corporation.
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

package v1alpha1

type event struct {
	reason    eventReason
	eventType eventType
}

func (e event) EventType() string {
	return string(e.eventType)
}

func (e event) Reason() string {
	return string(e.reason)
}

// eventType tracks the types supported by the Kubernetes EventRecorder interface in k8s.io/client-go/tools/record
type eventType string

const (
	EventTypeNormal  eventType = "Normal"
	EventTypeWarning eventType = "Warning"
)

// eventReason reflects distinct categorizations of events emitted by the stack controller.
type eventReason string

const (
	// Warnings
	Migrated                   eventReason = "Migrated"
	ConnectionFailure          eventReason = "ConnectionFailure"
	InstallationFailure        eventReason = "InstallationFailure"
	StackInitializationFailure eventReason = "StackInitializationFailure"
	UpdateFailed               eventReason = "UpdateFailed"

	// Normals
	Initialized     eventReason = "Initialized"
	UpdateExpired   eventReason = "UpdateExpired"
	UpdateSucceeded eventReason = "UpdateSucceeded"
)

func MigratedEvent() event {
	return event{eventType: EventTypeWarning, reason: Migrated}
}

func ConnectionFailureEvent() event {
	return event{eventType: EventTypeWarning, reason: ConnectionFailure}
}

func InstallationFailureEvent() event {
	return event{eventType: EventTypeWarning, reason: InstallationFailure}
}

func StackInitializationFailureEvent() event {
	return event{eventType: EventTypeWarning, reason: StackInitializationFailure}
}

func UpdateFailedEvent() event {
	return event{eventType: EventTypeWarning, reason: UpdateFailed}
}

func InitializedEvent() event {
	return event{eventType: EventTypeNormal, reason: Initialized}
}

func UpdateExpiredEvent() event {
	return event{eventType: EventTypeNormal, reason: UpdateExpired}
}

func UpdateSucceededEvent() event {
	return event{eventType: EventTypeNormal, reason: UpdateSucceeded}
}
