DELETE FROM job_steps WHERE id IN ('step-1-1', 'step-1-2', 'step-1-3', 'step-2-1', 'step-2-2', 'step-2-3');
DELETE FROM jobs WHERE id IN ('health-check', 'deploy-app');
DELETE FROM targets WHERE id = 'target-1';
