INSERT INTO targets (id, host, port, "user", key_path, tags)
VALUES ('target-1', '127.0.0.1', '22', 'keerthan', '/home/keerthan/.ssh/id_rsa', ARRAY['web-server', 'production'])
ON CONFLICT (id) DO NOTHING;

INSERT INTO jobs (id, name, target_filter)
VALUES 
  ('health-check', 'System Health Diagnostics', ARRAY['web-server', 'production']),
  ('deploy-app', 'Deploy Application Stack', ARRAY['web-server'])
ON CONFLICT (id) DO NOTHING;

INSERT INTO job_steps (id, job_id, step_order, type, attributes)
VALUES
  ('step-1-1', 'health-check', 1, 'command', '{"value": "echo ''Running Diagnostics...''"}'::jsonb),
  ('step-1-2', 'health-check', 2, 'command', '{"value": "df -h"}'::jsonb),
  ('step-1-3', 'health-check', 3, 'command', '{"value": "free -m"}'::jsonb),

  ('step-2-1', 'deploy-app', 1, 'file-copy', '{"source_path": "./config_demo.txt", "dest_path": "/tmp/gruntdeck_test/config_demo.txt"}'::jsonb),
  ('step-2-2', 'deploy-app', 2, 'script', '{"source_path": "./script_demo.sh", "args": ["hello", "world"]}'::jsonb),
  ('step-2-3', 'deploy-app', 3, 'job-ref', '{"job_id": "health-check"}'::jsonb)
ON CONFLICT (id) DO NOTHING;
