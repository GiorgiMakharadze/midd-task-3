# Race Condition Review

## Race Condition 1: Unsynchronized Read in GetStock

- **Code:** `GetStock` reads `s.products[productID]` and `product.Stock` with no locking.
- **What happens:** A concurrent `Reserve` call can modify `product.Stock` while `GetStock` reads it. This is a data race on the `Stock` field — the read may see a partially written value or a stale value.
- **Production scenario:** Goroutine A calls `GetStock("p1")` to display available inventory. Goroutine B calls `Reserve("p1", 5)` at the same time. Goroutine A reads `Stock` mid-write, returning an incorrect value. With the race detector enabled, this is flagged as a concurrent read/write on the same memory location.
- **Fix approach:** Protect `GetStock` with `sync.RWMutex.RLock`/`RUnlock`. Multiple readers can proceed concurrently, but readers block while a writer holds the lock.

## Race Condition 2: Unsynchronized Read-Check-Write in Reserve

- **Code:** `Reserve` reads `product.Stock`, compares it to `quantity`, and decrements it — all without any lock.
- **What happens:** Two goroutines can both read the same stock value, both pass the `Stock < quantity` check, and both decrement. This causes overselling: the final stock goes negative.
- **Production scenario:** Product has 1 unit in stock. Goroutine A and Goroutine B both call `Reserve("p1", 1)`. Both read `Stock == 1`, both pass the check, both decrement to 0. The product ends up at `Stock == -1` — one unit was sold that did not exist.
- **Fix approach:** Wrap the entire check-and-decrement in a single `sync.RWMutex.Lock`/`Unlock` critical section so the operation is atomic.

## Race Condition 3: Non-Atomic Check-Then-Apply in ReserveMultiple

- **Code:** `ReserveMultiple` first loops through all items to check stock, then loops again to apply decrements — with no locking at all.
- **What happens:** Between the validation loop and the apply loop (or even between iterations), another goroutine can modify stock. This breaks all-or-nothing semantics: the check may pass based on stale values, and partial decrements can oversell.
- **Production scenario:** Product A has 10 stock, Product B has 5 stock. Goroutine 1 calls `ReserveMultiple([{A, 8}, {B, 3}])`. Goroutine 2 calls `Reserve("A", 5)` between goroutine 1's check loop and apply loop. Goroutine 1's check saw A at 10 and passed. But by apply time, A is at 5. The decrement sets A to -3. Additionally, the map access itself (`s.products[...]`) is a data race with no synchronization.
- **Fix approach:** Hold a single write lock for the entire operation — validation and application — so the check-and-apply is atomic and no other goroutine can interleave.

> **Implementation note:** The fixed `ReserveMultiple` also aggregates requested quantities by ProductID before validation. Without aggregation, duplicate ProductIDs in a single request (e.g., `[{A, 8}, {A, 8}]` with `stock[A] = 10`) would each validate independently against the current stock and both pass, causing oversell within a single atomic call. Aggregating first ensures the total requested quantity per product is checked against stock.

## Race Condition 4: Local Mutex in SafeReserve

- **Code:** `SafeReserve` declares `var mu sync.Mutex` as a local variable inside the function body.
- **What happens:** Every call to `SafeReserve` creates a brand new mutex on the stack. Each goroutine locks and unlocks its own private mutex. There is zero mutual exclusion between concurrent calls — the shared `product.Stock` is still accessed without any real synchronization.
- **Production scenario:** 100 goroutines call `SafeReserve("p1", 1)` concurrently on a product with 50 stock. Each goroutine creates its own mutex, locks it, reads/writes `Stock`, and unlocks it. None of them ever contend with each other. The result is identical to having no lock at all — lost updates and overselling occur.
- **Fix approach:** The mutex must be shared across all calls. Place it as a field on the struct (e.g., `mu sync.RWMutex` on `InventoryService` or `SafeInventoryService`) so every goroutine contends on the same lock.
