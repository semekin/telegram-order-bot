package orders

import (
    "time"
)

type Order struct {
    ID          string    `json:"id"`
    UserID      int64     `json:"user_id"`
    Username    string    `json:"username"`
    Product     string    `json:"product"`
    Quantity    int       `json:"quantity"`
    Address     string    `json:"address"`
    Phone       string    `json:"phone"`
    CreatedAt   time.Time `json:"created_at"`
    Status      string    `json:"status"`
}

type OrderManager struct {
    orders []Order
}

func NewOrderManager() *OrderManager {
    return &OrderManager{
        orders: make([]Order, 0),
    }
}

func (om *OrderManager) CreateOrder(userID int64, username, product, address, phone string, quantity int) Order {
    order := Order{
        ID:        generateOrderID(),
        UserID:    userID,
        Username:  username,
        Product:   product,
        Quantity:  quantity,
        Address:   address,
        Phone:     phone,
        CreatedAt: time.Now(),
        Status:    "новый",
    }
    
    om.orders = append(om.orders, order)
    return order
}

func (om *OrderManager) GetOrders() []Order {
    return om.orders
}

func generateOrderID() string {
    return time.Now().Format("20060102150405")
}