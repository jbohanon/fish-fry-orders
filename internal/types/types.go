package types

import (
	"strconv"
	"time"

	pb "git.nonahob.net/jacob/fish-fry-orders/proto"
)

// DBSession represents an event session in the database
type DBSession struct {
	ID              int        `json:"id"`
	EventName       string     `json:"event_name"`
	StartedAt       time.Time  `json:"started_at"`
	ExpiresAt       time.Time  `json:"expires_at"`
	ClosedAt        *time.Time `json:"closed_at,omitempty"`
	Status          string     `json:"status"` // ACTIVE or CLOSED
	FinalOrderCount *int       `json:"final_order_count,omitempty"`
	FinalRevenue    *float64   `json:"final_revenue,omitempty"`
	Notes           string     `json:"notes,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// DBMenuItem represents a menu item in the database
type DBMenuItem struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Price        float64   `json:"price"`
	IsActive     bool      `json:"is_active"`
	DisplayOrder int       `json:"display_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DBOrder represents an order in the database
type DBOrder struct {
	ID                 int       `json:"id"`
	SessionID          int       `json:"session_id"`
	DailyOrderNumber   int       `json:"daily_order_number"`
	VehicleDescription string    `json:"vehicle_description"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// DBOrderItem represents an order item in the database
type DBOrderItem struct {
	ID         string    `json:"id"`
	OrderID    int       `json:"order_id"`
	MenuItemID string    `json:"menu_item_id"`
	ItemName   string    `json:"item_name"`   // Captured at order time
	UnitPrice  float64   `json:"unit_price"`  // Captured at order time
	Quantity   int32     `json:"quantity"`
	CreatedAt  time.Time `json:"created_at"`
}

// DBChatMessage represents a chat message in the database
type DBChatMessage struct {
	ID         string    `json:"id"`
	OrderID    int       `json:"order_id"`
	Content    string    `json:"content"`
	SenderRole string    `json:"sender_role"`
	CreatedAt  time.Time `json:"created_at"`
}

// DBOrderStatistics represents statistics about orders
type DBOrderStatistics struct {
	ItemCounts            map[string]int32 `json:"item_counts"`
	AverageTimeToComplete time.Duration    `json:"average_completion_time"`
	TotalOrders           int32            `json:"total_orders"`
}

// ToProtoMenuItem converts a database MenuItem to a protobuf MenuItem
func (m *DBMenuItem) ToProto() *pb.MenuItem {
	return &pb.MenuItem{
		Id:       m.ID,
		Name:     m.Name,
		Price:    m.Price,
		IsActive: m.IsActive,
	}
}

// FromProtoMenuItem converts a protobuf MenuItem to a database MenuItem
func FromProtoMenuItem(m *pb.MenuItem) *DBMenuItem {
	return &DBMenuItem{
		ID:       m.Id,
		Name:     m.Name,
		Price:    m.Price,
		IsActive: m.IsActive,
	}
}

// ToProtoOrder converts a database Order to a protobuf Order
func (o *DBOrder) ToProto() *pb.Order {
	return &pb.Order{
		Id:                 strconv.Itoa(o.ID),
		VehicleDescription: o.VehicleDescription,
		Status:             pb.OrderStatus(pb.OrderStatus_value[o.Status]),
		CreatedAt:          o.CreatedAt.Unix(),
		UpdatedAt:          o.UpdatedAt.Unix(),
	}
}

// FromProtoOrder converts a protobuf Order to a database Order
func FromProtoOrder(o *pb.Order) *DBOrder {
	id, _ := strconv.Atoi(o.Id) // Ignore error, will be 0 if invalid
	return &DBOrder{
		ID:                 id,
		VehicleDescription: o.VehicleDescription,
		Status:             o.Status.String(),
		CreatedAt:          time.Unix(o.CreatedAt, 0),
		UpdatedAt:          time.Unix(o.UpdatedAt, 0),
	}
}

// ToProtoOrderItem converts a database OrderItem to a protobuf OrderItem
func (i *DBOrderItem) ToProto() *pb.OrderItem {
	return &pb.OrderItem{
		MenuItemId: i.MenuItemID,
		Quantity:   i.Quantity,
	}
}

// FromProtoOrderItem converts a protobuf OrderItem to a database OrderItem
func FromProtoOrderItem(i *pb.OrderItem) *DBOrderItem {
	return &DBOrderItem{
		MenuItemID: i.MenuItemId,
		Quantity:   i.Quantity,
	}
}

// ToProtoChatMessage converts a database ChatMessage to a protobuf ChatMessage
func (m *DBChatMessage) ToProto() *pb.ChatMessage {
	return &pb.ChatMessage{
		Id:         m.ID,
		OrderId:    strconv.Itoa(m.OrderID),
		Content:    m.Content,
		SenderRole: m.SenderRole,
		CreatedAt:  m.CreatedAt.Unix(),
	}
}

// FromProtoChatMessage converts a protobuf ChatMessage to a database ChatMessage
func FromProtoChatMessage(m *pb.ChatMessage) *DBChatMessage {
	orderID, _ := strconv.Atoi(m.OrderId) // Ignore error, will be 0 if invalid
	return &DBChatMessage{
		ID:         m.Id,
		OrderID:    orderID,
		Content:    m.Content,
		SenderRole: m.SenderRole,
		CreatedAt:  time.Unix(m.CreatedAt, 0),
	}
}

// ToProtoOrderStatistics converts a database OrderStatistics to a protobuf OrderStatistics
func (s *DBOrderStatistics) ToProto() *pb.OrderStatistics {
	return &pb.OrderStatistics{
		ItemCounts:            s.ItemCounts,
		AverageCompletionTime: s.AverageTimeToComplete.Seconds(),
		TotalOrders:           s.TotalOrders,
	}
}

// FromProtoOrderStatistics converts a protobuf OrderStatistics to a database OrderStatistics
func FromProtoOrderStatistics(s *pb.OrderStatistics) *DBOrderStatistics {
	return &DBOrderStatistics{
		ItemCounts:            s.ItemCounts,
		AverageTimeToComplete: time.Duration(s.AverageCompletionTime * float64(time.Second)),
		TotalOrders:           s.TotalOrders,
	}
}
