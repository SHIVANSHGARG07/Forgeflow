-- 000001_init.down.sql: 000001_init.up.sql ko undo karta hai.
-- Reverse order me DROP (foreign keys ki wajah se) - tasks pehle, phir workflow_runs, phir workflows.

DROP TABLE tasks;
DROP TABLE workflow_runs;
DROP TABLE workflows;
