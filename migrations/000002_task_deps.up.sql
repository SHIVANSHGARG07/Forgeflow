-- 000002_task_deps.up.sql: task dependencies (DAG edges) store karne ke liye.
-- task_id woh task hai jo depends_on_id wale task ke SUCCEEDED hone ka wait karega.

CREATE TABLE task_deps (
  task_id       UUID NOT NULL REFERENCES tasks(id),
  depends_on_id UUID NOT NULL REFERENCES tasks(id),
  PRIMARY KEY (task_id, depends_on_id)
);
