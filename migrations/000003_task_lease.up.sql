-- 000003_task_lease.up.sql: tasks ko lease dene ke liye columns.
-- lease_owner       -> kaunsa worker (worker_id) abhi is task ka "malik" hai
-- lease_expires_at  -> kab tak ye ownership valid hai, iske baad reaper
--                      is task ko wapas READY kar dega (worker crash maan ke)

ALTER TABLE tasks ADD COLUMN lease_owner TEXT;
ALTER TABLE tasks ADD COLUMN lease_expires_at TIMESTAMPTZ;
