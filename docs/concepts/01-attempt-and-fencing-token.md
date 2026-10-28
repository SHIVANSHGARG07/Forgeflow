# Concept: `attempt` number and Fencing Token

## The problem — "Zombie Worker"

1. Worker-1 polls a task → gets `task-X` with `attempt = 1`.
2. Worker-1 starts working, but is very slow (network delay, long GC pause, etc.) — it is NOT actually dead, just slow.
3. Scheduler decides (based on a timeout) that Worker-1 must have crashed, and marks `task-X` back to `READY`.
4. Worker-2 polls → gets the same `task-X`, now with `attempt = 2`.
5. Worker-2 finishes the task and reports success.
6. A little later, the "zombie" Worker-1 (which was never actually dead) finally finishes its old work too, and also reports success — but with the OLD `attempt = 1`.

**Danger:** If we don't check `attempt`, Worker-1's late/stale report could incorrectly overwrite or conflict with Worker-2's correct, newer result. Two workers end up fighting over the same task.

This situation — an old worker "waking up" and sending outdated data after the system has already moved on — is called the **Zombie Worker problem** (also related to **split-brain**).

## The solution — Fencing Token

**Rule:** Only accept a result from the worker whose `attempt` number matches the CURRENT `attempt` number stored in the database. Reject (ignore) anything older — even if it says "success".

The `attempt` number itself acts as the **fencing token** — a version number that increases every time a task is re-claimed.

### How it looks in SQL (conceptually)

```sql
UPDATE tasks
SET status = 'DONE'
WHERE id = $1 AND attempt = $2   -- fencing check
```

- `$2` is the attempt number the worker believes it has.
- If the DB's current `attempt` has already moved ahead (e.g. DB has 2, worker says 1), the `WHERE` clause matches **zero rows** — the update silently does nothing.
- The scheduler can check "0 rows updated" to know the result was stale and should be ignored.

## Real-world analogy

Think of booking a meeting room:
- You book it and get **version 1** of the booking.
- Your phone freezes; you think the booking failed.
- Someone else books the same room, creating **version 2**.
- Later your phone comes back and tries to act using your old **version 1** token (e.g. "cancel my booking").
- The system says: "that token is outdated, the current version is 2 — request rejected."

That's fencing: only the latest/current token is allowed to act; old tokens are automatically invalid.

## Why this matters in distributed systems

Guaranteeing a message is delivered "exactly once" is very hard in distributed systems — networks fail, retries happen, workers can appear dead and come back. The practical approach instead is:

> Messages can arrive **more than once** (at-least-once delivery), but the system is designed so duplicate/stale messages can never cause harm.

The `attempt` field does two jobs at once:
1. **Fencing** — reject stale/duplicate results from old workers (explained above).
2. **Retry counting** — track how many times a task has been attempted, so we know when to give up and mark it `FAILED` (compared against `max_attempts`).

## One-line summary

> `attempt` is a version number for a task. Every time a task is re-claimed, the version goes up. Only the result matching the CURRENT version is trusted — anything older is a stale/zombie result and gets ignored.
