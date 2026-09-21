package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/maddydevel/HiveStack/internal/db"
)

// EventType identifies the kind of event that occurred.
type EventType string

// Severity classifies how important an event is.
type Severity string

// Event type constants.
const (
	VMCreated               EventType = "vm_created"
	VMDeleted               EventType = "vm_deleted"
	VMStarted               EventType = "vm_started"
	VMStopped               EventType = "vm_stopped"
	VMRestarted             EventType = "vm_restarted"
	VMMigrated              EventType = "vm_migrated"
	HostRegistered          EventType = "host_registered"
	HostDecommissioned      EventType = "host_decommissioned"
	HostMaintenanceEnter    EventType = "host_maintenance_enter"
	HostMaintenanceExit     EventType = "host_maintenance_exit"
	ComplianceCheckPassed   EventType = "compliance_check_passed"
	ComplianceCheckFailed   EventType = "compliance_check_failed"
	ComplianceDriftDetected EventType = "compliance_drift_detected"
	BackupCreated           EventType = "backup_created"
	BackupRestored          EventType = "backup_restored"
	BackupFailed            EventType = "backup_failed"
	NodeRegistered          EventType = "node_registered"
	NodeOffline             EventType = "node_offline"
)

// Severity constants.
const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// String implements fmt.Stringer for EventType.
func (e EventType) String() string {
	return string(e)
}

// Event represents an audit-log / lifecycle event. Its field layout matches the
// db.Event row — see internal/db/models.go.
type Event struct {
	ID           string          `json:"id"`
	TenantID     string          `json:"tenant_id"`
	Type         EventType       `json:"type"`
	Severity     Severity        `json:"severity"`
	Message      string          `json:"message"`
	ActorType    string          `json:"actor_type"`
	ActorID      string          `json:"actor_id"`
	ActorName    string          `json:"actor_name"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	ResourceName string          `json:"resource_name"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    string          `json:"created_at"`
}

// EventFilter describes optional constraints for List queries.
type EventFilter struct {
	Type     *EventType // exact match
	Severity *Severity  // exact match
	After    *string    // ISO-8601 timestamp; events after this time
	Before   *string    // ISO-8601 timestamp; events before this time
	Limit    int        // max rows to return
}

// EventStore persists and retrieves Event records from the backing datastore.
type EventStore interface {
	Create(ctx context.Context, event *Event) (string, error)
	List(ctx context.Context, tenantID string, filter EventFilter) ([]*Event, error)
	Get(ctx context.Context, id string) (*Event, error)
}

// Subscription represents an active subscriber to the EventPublisher.
type Subscription interface {
	Events() <-chan *Event
	Close() error
}

type subscription struct {
	ch   chan *Event
	done chan struct{}
}

func (s *subscription) Events() <-chan *Event {
	return s.ch
}

func (s *subscription) Close() error {
	close(s.done)
	return nil
}

// EventPublisher broadcasts events to registered subscribers.
type EventPublisher interface {
	Publish(ctx context.Context, event *Event) error
	Subscribe() <-chan *Event
}

// EventManager implements EventStore and EventPublisher. It writes every event
// to the database and fans it out through an internal channel to subscribers.
type EventManager struct {
	db          *db.DB
	pubCh       chan *Event
	subscribers map[string]chan *Event
	mu          sync.RWMutex
}

// NewEventManager returns a ready-to-use EventManager.
func NewEventManager(db *db.DB) *EventManager {
	return &EventManager{
		db:          db,
		pubCh:       make(chan *Event, 256),
		subscribers: make(map[string]chan *Event),
	}
}

// Create inserts the event into the database and returns its new ID.
func (m *EventManager) Create(ctx context.Context, event *Event) (string, error) {
	id, err := m.db.CreateEvent(ctx, toDBEvent(event))
	if err != nil {
		return "", fmt.Errorf("events.Create: %w", err)
	}
	return id, nil
}

// List returns events for the given tenant, honouring the supplied filter.
func (m *EventManager) List(ctx context.Context, tenantID string, filter EventFilter) ([]*Event, error) {
	// db.ListEvents currently only supports a simple limit; we build a DB-side
	// limit from the filter and apply type/severity/time filtering in-memory
	// because the existing signature does not expose those parameters.
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	dbEvents, err := m.db.ListEvents(ctx, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("events.List: %w", err)
	}

	filtered := make([]*Event, 0, len(dbEvents))
	for i := range dbEvents {
		e := toDomainEvent(&dbEvents[i])
		if filter.Type != nil && e.Type != *filter.Type {
			continue
		}
		if filter.Severity != nil && e.Severity != *filter.Severity {
			continue
		}
		if filter.After != nil && e.CreatedAt <= *filter.After {
			continue
		}
		if filter.Before != nil && e.CreatedAt >= *filter.Before {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered, nil
}

// Get retrieves a single event by ID.
func (m *EventManager) Get(ctx context.Context, id string) (*Event, error) {
	row := m.db.QueryRowContext(ctx,
		"SELECT id, tenant_id, type, severity, message,"+
			" actor_type, actor_id, actor_name,"+
			" resource_type, resource_id, resource_name,"+
			" metadata, created_at"+
			" FROM event WHERE id = $1", id)
	var e db.Event
	err := row.Scan(&e.ID, &e.TenantID, &e.Type, &e.Severity, &e.Message,
		&e.ActorType, &e.ActorID, &e.ActorName,
		&e.ResourceType, &e.ResourceID, &e.ResourceName,
		&e.Metadata, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("events.Get: %w", err)
	}
	return toDomainEvent(&e), nil
}

// Publish inserts the event into the database and broadcasts it to all
// subscribers via pubCh.
func (m *EventManager) Publish(ctx context.Context, event *Event) error {
	id, err := m.db.CreateEvent(ctx, toDBEvent(event))
	if err != nil {
		return fmt.Errorf("events.Publish: %w", err)
	}
	event.ID = id

	m.pubCh <- event
	return nil
}

// Subscribe creates a new channel and starts a goroutine that reads from
// pubCh, returning a channel that receives all published events.
func (m *EventManager) Subscribe() <-chan *Event {
	m.mu.Lock()
	ch := make(chan *Event, 256)
	id := fmt.Sprintf("sub-%d", len(m.subscribers))
	m.subscribers[id] = ch
	m.mu.Unlock()

	go func() {
		defer close(ch)
		for event := range m.pubCh {
			ch <- event
		}
	}()

	return ch
}

// NewEvent is a helper that constructs an *Event with the given fields.
func NewEvent(
	tenantID string,
	eventType EventType,
	severity Severity,
	message string,
	actorType, actorID, actorName,
	resourceType, resourceID, resourceName string,
	metadata interface{},
) *Event {
	var raw json.RawMessage
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			raw = json.RawMessage(`null`)
		} else {
			raw = b
		}
	}

	return &Event{
		TenantID:     tenantID,
		Type:         eventType,
		Severity:     severity,
		Message:      message,
		ActorType:    actorType,
		ActorID:      actorID,
		ActorName:    actorName,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		Metadata:     raw,
	}
}

// ---------- internal helpers ----------

// toDBEvent converts our domain Event to the db.Event used by repositories.go.
func toDBEvent(e *Event) *db.Event {
	return &db.Event{
		TenantID:     e.TenantID,
		Type:         string(e.Type),
		Severity:     string(e.Severity),
		Message:      e.Message,
		ActorType:    e.ActorType,
		ActorID:      e.ActorID,
		ActorName:    e.ActorName,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		ResourceName: e.ResourceName,
		Metadata:     e.Metadata,
	}
}

// toDomainEvent converts a db.Event back to our domain Event.
func toDomainEvent(d *db.Event) *Event {
	var createdAt string
	if !d.CreatedAt.IsZero() {
		createdAt = d.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return &Event{
		ID:           d.ID,
		TenantID:     d.TenantID,
		Type:         EventType(d.Type),
		Severity:     Severity(d.Severity),
		Message:      d.Message,
		ActorType:    d.ActorType,
		ActorID:      d.ActorID,
		ActorName:    d.ActorName,
		ResourceType: d.ResourceType,
		ResourceID:   d.ResourceID,
		ResourceName: d.ResourceName,
		Metadata:     d.Metadata,
		CreatedAt:    createdAt,
	}
}
