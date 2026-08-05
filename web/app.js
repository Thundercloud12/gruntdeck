let currentExecutionID = null;
let logPollInterval = null;

document.addEventListener('DOMContentLoaded', () => {
  setupNavigation();
  loadJobs();
});

// Tab Navigation
function setupNavigation() {
  const tabs = document.querySelectorAll('.nav-tab');
  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

      tab.classList.add('active');
      const targetId = tab.getAttribute('data-tab');
      document.getElementById(targetId).classList.add('active');

      if (targetId === 'jobs-tab') loadJobs();
      if (targetId === 'executions-tab') loadExecutions();
      if (targetId === 'targets-tab') loadTargets();
    });
  });
}

// 1. Fetch & Render Jobs
async function loadJobs() {
  const tbody = document.getElementById('jobs-table-body');
  try {
    const res = await fetch('/api/v1/jobs');
    if (!res.ok) throw new Error('Failed to fetch jobs');
    const jobs = await res.json();

    if (!jobs || jobs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="text-center loading-text">No jobs found in database.</td></tr>';
      return;
    }

    tbody.innerHTML = jobs.map(job => {
      const filters = (job.TargetFilter || []).map(t => `<span class="badge badge-tag">${t}</span>`).join('') || '<span class="text-muted">None</span>';
      const steps = (job.Steps || []).map(s => `<span class="badge badge-step">${s.Type}</span>`).join('') || '<span class="text-muted">None</span>';
      const optionsCount = (job.Options || []).length;
      const optionsBadge = optionsCount > 0 ? `<span class="badge badge-step" style="background: rgba(139, 92, 246, 0.2); color: #a78bfa;">⚙️ ${optionsCount} Option(s)</span>` : '';

      return `
        <tr>
          <td>
            <strong>${escapeHTML(job.Name || job.ID)}</strong> ${optionsBadge}
            <div style="font-size: 0.75rem; color: var(--text-muted); font-family: var(--font-mono);">${job.ID}</div>
          </td>
          <td>${filters}</td>
          <td>${steps}</td>
          <td class="text-right">
            <button class="btn btn-primary btn-xs" onclick="runJob('${job.ID}')">
              ▶ Run Job
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="4" class="text-center loading-text" style="color: var(--danger);">Error loading jobs: ${err.message}</td></tr>`;
  }
}

// 2. Trigger Execution with Runtime Options
async function runJob(jobID) {
  let optionValues = {};
  try {
    const jobRes = await fetch(`/api/v1/jobs/${encodeURIComponent(jobID)}`);
    if (jobRes.ok) {
      const job = await jobRes.json();
      if (job.Options && job.Options.length > 0) {
        for (const opt of job.Options) {
          const promptMsg = `Job Option: ${opt.name}\n${opt.description ? opt.description + '\n' : ''}${opt.required ? '(Required) ' : ''}Default: '${opt.default_value}'${opt.choices && opt.choices.length ? '\nChoices: ' + opt.choices.join(', ') : ''}`;
          const input = prompt(promptMsg, opt.default_value);
          if (input === null) return; // User cancelled
          if (opt.required && !input && !opt.default_value) {
            alert(`Option '${opt.name}' is required.`);
            return;
          }
          if (input !== '') {
            optionValues[opt.name] = input;
          }
        }
      }
    }
  } catch (err) {
    console.warn("Failed to pre-fetch job options, running with default", err);
  }

  try {
    const res = await fetch(`/api/v1/jobs/${encodeURIComponent(jobID)}/run`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ options: optionValues })
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to enqueue job');
    }

    const data = await res.json();
    alert(`🚀 Execution enqueued successfully!\nExecution ID: ${data.execution_id}`);

    // Switch to Executions tab and open modal
    document.querySelector('[data-tab="executions-tab"]').click();
    openLogModal(data.execution_id);
  } catch (err) {
    alert(`❌ Failed to run job: ${err.message}`);
  }
}

// 3. Fetch & Render Executions
async function loadExecutions() {
  const tbody = document.getElementById('executions-table-body');
  try {
    const res = await fetch('/api/v1/executions');
    if (!res.ok) throw new Error('Failed to fetch executions');
    const executions = await res.json();

    if (!executions || executions.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" class="text-center loading-text">No executions recorded yet.</td></tr>';
      return;
    }

    tbody.innerHTML = executions.map(ex => {
      const statusClass = (ex.Status || 'queued').toLowerCase();
      const started = ex.StartedAt ? new Date(ex.StartedAt).toLocaleString() : '-';

      return `
        <tr>
          <td><span class="badge badge-mono">${ex.ID}</span></td>
          <td><strong>${escapeHTML(ex.JobID)}</strong></td>
          <td><span class="badge badge-status ${statusClass}">${ex.Status}</span></td>
          <td>${ex.TargetsTotal || 1} node(s)</td>
          <td style="font-size: 0.8rem; color: var(--text-muted);">${started}</td>
          <td class="text-right">
            <button class="btn btn-secondary btn-xs" onclick="openLogModal('${ex.ID}')">
              📄 View Logs
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="6" class="text-center loading-text" style="color: var(--danger);">Error loading executions: ${err.message}</td></tr>`;
  }
}

// 4. Fetch & Render Targets
async function loadTargets() {
  const tbody = document.getElementById('targets-table-body');
  try {
    const res = await fetch('/api/v1/targets');
    if (!res.ok) throw new Error('Failed to fetch targets');
    const targets = await res.json();

    if (!targets || targets.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="text-center loading-text">No target nodes registered.</td></tr>';
      return;
    }

    tbody.innerHTML = targets.map(t => {
      const tags = (t.Tags || []).map(tag => `<span class="badge badge-tag">${tag}</span>`).join('') || '<span class="text-muted">None</span>';
      return `
        <tr>
          <td><span class="badge badge-mono">${t.ID}</span></td>
          <td><strong>${escapeHTML(t.Host)}</strong></td>
          <td>${t.Port || 22}</td>
          <td>${escapeHTML(t.User)}</td>
          <td>${tags}</td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="5" class="text-center loading-text" style="color: var(--danger);">Error loading target nodes: ${err.message}</td></tr>`;
  }
}

// 5. Terminal Log Console Modal
function openLogModal(executionID) {
  currentExecutionID = executionID;
  document.getElementById('modal-execution-id').innerText = `ID: ${executionID}`;
  document.getElementById('log-modal').classList.remove('hidden');
  fetchLogsForModal(executionID);

  if (logPollInterval) clearInterval(logPollInterval);
  logPollInterval = setInterval(() => {
    if (currentExecutionID === executionID) {
      fetchLogsForModal(executionID);
    }
  }, 2000);
}

function closeLogModal() {
  document.getElementById('log-modal').classList.add('hidden');
  currentExecutionID = null;
  if (logPollInterval) {
    clearInterval(logPollInterval);
    logPollInterval = null;
  }
}

function refreshCurrentLogs() {
  if (currentExecutionID) {
    fetchLogsForModal(currentExecutionID);
  }
}

async function fetchLogsForModal(executionID) {
  const outputElem = document.getElementById('terminal-log-output');
  const statusElem = document.getElementById('terminal-status');

  try {
    const res = await fetch(`/api/v1/executions/${encodeURIComponent(executionID)}/logs`);
    if (!res.ok) throw new Error('Failed to fetch logs');
    const logs = await res.json();

    if (!logs || logs.length === 0) {
      outputElem.textContent = '⏳ Waiting for log stream from executor daemon...';
      statusElem.textContent = 'Status: Waiting for output';
      return;
    }

    statusElem.textContent = `Status: ${logs.length} log entry(ies) recorded`;
    outputElem.textContent = logs.map(entry => {
      const time = new Date(entry.Timestamp).toLocaleTimeString();
      return `[${time}] [${entry.TargetID}] ${entry.Message}`;
    }).join('\n');

    outputElem.scrollTop = outputElem.scrollHeight;
  } catch (err) {
    outputElem.textContent = `Error loading logs: ${err.message}`;
  }
}

function escapeHTML(str) {
  if (!str) return '';
  return str.replace(/[&<>'"]/g, 
    tag => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[tag] || tag)
  );
}
