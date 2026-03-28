# Answers

## Q1: Why is the mutex in SafeReserve completely useless?

The line `var mu sync.Mutex` declares a **new, local mutex on the stack** every time `SafeReserve` is called. Each goroutine gets its own independent mutex instance. Goroutine A locks mutex-A, goroutine B locks mutex-B — they never contend with each other.

A mutex only provides mutual exclusion when **all goroutines lock the same mutex instance**. Since the local variable is created fresh per call, no two goroutines ever compete for the same lock. The shared `product.Stock` field is read and written without any real synchronization, making this functionally identical to having no mutex at all.

The fix is to make the mutex a **field on the struct** so it is shared across all method calls.

## Q2: What can happen with per-product locks when reserving A then B vs B then A?

This is a classic **deadlock** scenario:

1. Goroutine 1 locks Product A, then tries to lock Product B.
2. Goroutine 2 locks Product B, then tries to lock Product A.
3. Goroutine 1 holds A and waits for B. Goroutine 2 holds B and waits for A.
4. Neither can proceed — the program hangs forever.

**Prevention strategies:**

- **Simplest (used in this task):** Use a single service-wide `sync.RWMutex` instead of per-product locks. There is only one lock, so deadlock from lock ordering is impossible.
- **General alternative:** If per-product locks are needed for throughput, always acquire locks in a **canonical order** (e.g., sorted by product ID). Both goroutines would lock A first, then B, eliminating the circular wait condition.

## Q3: Why is the "unlock early" fix worse than no locks?

The code unlocks after reading the product pointer, checks stock without the lock, then re-locks to decrement. This creates a **time-of-check-to-time-of-use (TOCTOU)** bug:

1. Goroutine A locks, reads product (stock = 1), unlocks.
2. Goroutine B locks, reads product (stock = 1), unlocks.
3. Goroutine A sees `stock (1) >= quantity (1)`, passes the check.
4. Goroutine B sees `stock (1) >= quantity (1)`, passes the check.
5. Goroutine A locks, decrements stock to 0, unlocks.
6. Goroutine B locks, decrements stock to -1, unlocks.

**Result:** Stock is now negative. The invariant `Stock >= 0` is violated.

This is **worse than no locks** because it gives **false confidence**. A developer sees `Lock`/`Unlock` calls and assumes the operation is safe. In reality, the critical invariant (check-and-update must be atomic) is still broken. Without any locks, the code is at least obviously unsafe. With these locks, the bug is hidden behind an illusion of correctness.

## Q4: Does passing `-race` with no warnings mean the code is race-free?

**No.** The `-race` flag enables Go's runtime race detector, which is a **dynamic analysis tool**. It only detects races that **actually occur during execution** of the specific test run.

Limitations:

- It can only catch races on **code paths that are exercised**. If a test does not trigger concurrent access to a particular shared variable, the race detector will not report it.
- It is **non-deterministic** — a race that depends on specific goroutine scheduling may not manifest in every run.
- It does not detect **logical concurrency bugs** like TOCTOU violations, deadlocks, or broken atomicity where the individual memory accesses happen to be serialized by timing.
- It cannot prove the **absence** of races — only the presence of races it observes.

A clean `-race` run increases confidence but is not a proof of correctness. Thorough concurrent tests with high goroutine counts, combined with code review of lock discipline, are needed to build real confidence.
