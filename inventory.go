package inventory

import (
	"errors"
	"sync"
)

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrInvalidQuantity   = errors.New("quantity must be positive")
)

type Product struct {
	ID    string
	Name  string
	Stock int
}

type ReserveItem struct {
	ProductID string
	Quantity  int
}

type SafeInventoryService struct {
	mu       sync.RWMutex
	products map[string]*Product
}

func NewSafeInventoryService(products map[string]*Product) *SafeInventoryService {
	copied := make(map[string]*Product, len(products))
	for id, p := range products {
		if p == nil {
			continue
		}
		clone := *p
		copied[id] = &clone
	}
	return &SafeInventoryService{products: copied}
}

func (s *SafeInventoryService) GetStock(productID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	product := s.products[productID]
	if product == nil {
		return 0
	}
	return product.Stock
}

func (s *SafeInventoryService) Reserve(productID string, quantity int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	product := s.products[productID]
	if product == nil {
		return ErrProductNotFound
	}

	if product.Stock < quantity {
		return ErrInsufficientStock
	}

	product.Stock -= quantity
	return nil
}

func (s *SafeInventoryService) ReserveMultiple(items []ReserveItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	totals := make(map[string]int, len(items))
	for _, item := range items {
		if item.Quantity <= 0 {
			return ErrInvalidQuantity
		}
		if s.products[item.ProductID] == nil {
			return ErrProductNotFound
		}
		totals[item.ProductID] += item.Quantity
	}

	for productID, qty := range totals {
		if s.products[productID].Stock < qty {
			return ErrInsufficientStock
		}
	}

	for productID, qty := range totals {
		s.products[productID].Stock -= qty
	}

	return nil
}
