-- 000003_task_lease.down.sql: 000003_task_lease.up.sql ko undo karta hai.

ALTER TABLE tasks DROP COLUMN lease_owner;
ALTER TABLE tasks DROP COLUMN lease_expires_at;
