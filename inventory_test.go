package inventory

import (
	"sync"
	"testing"
)

func TestReserve_ConcurrentOversell(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{
		"p1": {ID: "p1", Name: "Widget", Stock: 100},
	})

	const goroutines = 200
	var (
		wg        sync.WaitGroup
		start     sync.WaitGroup
		successMu sync.Mutex
		successes int
		failures  int
	)

	start.Add(1)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			start.Wait()
			err := svc.Reserve("p1", 1)

			successMu.Lock()
			defer successMu.Unlock()
			if err == nil {
				successes++
			} else {
				failures++
			}
		}()
	}
	start.Done()
	wg.Wait()

	if successes != 100 {
		t.Errorf("expected 100 successes, got %d", successes)
	}
	if failures != 100 {
		t.Errorf("expected 100 failures, got %d", failures)
	}
	if stock := svc.GetStock("p1"); stock != 0 {
		t.Errorf("expected stock 0, got %d", stock)
	}
}

func TestReserveMultiple_Atomicity(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{
		"a": {ID: "a", Name: "Product A", Stock: 10},
		"b": {ID: "b", Name: "Product B", Stock: 5},
	})

	err := svc.ReserveMultiple([]ReserveItem{
		{ProductID: "a", Quantity: 8},
		{ProductID: "b", Quantity: 8},
	})
	if err != ErrInsufficientStock {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}

	if stock := svc.GetStock("a"); stock != 10 {
		t.Errorf("expected product A stock 10, got %d", stock)
	}
	if stock := svc.GetStock("b"); stock != 5 {
		t.Errorf("expected product B stock 5, got %d", stock)
	}
}

func TestGetStock_NotFound(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{})

	if stock := svc.GetStock("missing"); stock != 0 {
		t.Errorf("expected 0 for missing product, got %d", stock)
	}
}

func TestReserve_ProductNotFound(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{})

	err := svc.Reserve("missing", 1)
	if err != ErrProductNotFound {
		t.Errorf("expected ErrProductNotFound, got %v", err)
	}
}

func TestReserveMultiple_DuplicateProductIDs_AggregatedAtomically(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{
		"a": {ID: "a", Name: "Product A", Stock: 10},
	})

	err := svc.ReserveMultiple([]ReserveItem{
		{ProductID: "a", Quantity: 8},
		{ProductID: "a", Quantity: 8},
	})
	if err != ErrInsufficientStock {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}

	if stock := svc.GetStock("a"); stock != 10 {
		t.Errorf("expected product A stock 10, got %d", stock)
	}
}

func TestReserve_InvalidQuantity(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{
		"a": {ID: "a", Name: "Product A", Stock: 10},
	})

	for _, qty := range []int{0, -1, -100} {
		if err := svc.Reserve("a", qty); err != ErrInvalidQuantity {
			t.Errorf("Reserve(qty=%d): expected ErrInvalidQuantity, got %v", qty, err)
		}
	}

	if stock := svc.GetStock("a"); stock != 10 {
		t.Errorf("expected stock 10, got %d", stock)
	}
}

func TestReserveMultiple_InvalidQuantity(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{
		"a": {ID: "a", Name: "Product A", Stock: 10},
	})

	err := svc.ReserveMultiple([]ReserveItem{
		{ProductID: "a", Quantity: 0},
	})
	if err != ErrInvalidQuantity {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}

	if stock := svc.GetStock("a"); stock != 10 {
		t.Errorf("expected stock 10, got %d", stock)
	}
}

func TestNewSafeInventoryService_NilProductSkipped(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{
		"a": {ID: "a", Name: "Product A", Stock: 5},
		"b": nil,
	})

	if stock := svc.GetStock("a"); stock != 5 {
		t.Errorf("expected stock 5, got %d", stock)
	}
	if stock := svc.GetStock("b"); stock != 0 {
		t.Errorf("expected 0 for nil product, got %d", stock)
	}
}

func TestReserveMultiple_ProductNotFound(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{
		"a": {ID: "a", Name: "Product A", Stock: 10},
	})

	err := svc.ReserveMultiple([]ReserveItem{
		{ProductID: "a", Quantity: 1},
		{ProductID: "missing", Quantity: 1},
	})
	if err != ErrProductNotFound {
		t.Errorf("expected ErrProductNotFound, got %v", err)
	}

	if stock := svc.GetStock("a"); stock != 10 {
		t.Errorf("expected product A stock 10, got %d", stock)
	}
}

func TestReserveMultiple_ConcurrentSafety(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{
		"a": {ID: "a", Name: "Product A", Stock: 30},
		"b": {ID: "b", Name: "Product B", Stock: 30},
	})

	const goroutines = 100
	var (
		wg, start sync.WaitGroup
		successMu sync.Mutex
		successes int
		failures  int
	)
	start.Add(1)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			start.Wait()
			err := svc.ReserveMultiple([]ReserveItem{
				{ProductID: "a", Quantity: 1},
				{ProductID: "b", Quantity: 1},
			})

			successMu.Lock()
			defer successMu.Unlock()
			if err == nil {
				successes++
			} else {
				failures++
			}
		}()
	}
	start.Done()
	wg.Wait()

	if successes != 30 {
		t.Errorf("expected 30 successes, got %d", successes)
	}
	if failures != 70 {
		t.Errorf("expected 70 failures, got %d", failures)
	}
	a := svc.GetStock("a")
	b := svc.GetStock("b")
	if a != b {
		t.Errorf("stocks diverged: a=%d b=%d", a, b)
	}
	if a != 0 {
		t.Errorf("expected stock 0, got a=%d b=%d", a, b)
	}
}
