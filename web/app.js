let currentExecutionID = null;
let logPollInterval = null;

document.addEventListener('DOMContentLoaded', async () => {
  await checkUserSession();
  setupNavigation();
  loadJobs();
});

// Check Session & Load User Profile
async function checkUserSession() {
  try {
    const res = await fetch('/api/v1/auth/me');
    if (!res.ok) {
      window.location.href = '/login.html';
      return;
    }
    const user = await res.json();
    const userBadge = document.getElementById('user-badge');
    if (userBadge && user.username) {
      userBadge.textContent = `👤 ${user.username}`;
    }
  } catch (err) {
    window.location.href = '/login.html';
  }
}

// Handle User Logout
async function handleLogout() {
  try {
    await fetch('/api/v1/auth/logout', { method: 'POST' });
  } catch (err) {
    console.error("Logout error", err);
  } finally {
    window.location.href = '/login.html';
  }
}

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
      if (targetId === 'schedules-tab') loadSchedules();
      if (targetId === 'executions-tab') loadExecutions();
      if (targetId === 'targets-tab') loadTargets();
      if (targetId === 'credentials-tab') loadCredentials();
    });
  });
}

// 1. Fetch & Render Jobs
async function loadJobs() {
  const tbody = document.getElementById('jobs-table-body');
  try {
    const res = await fetch('/api/v1/jobs');
    if (!res.ok) {
      if (res.status === 401) { window.location.href = '/login.html'; return; }
      throw new Error('Failed to fetch jobs');
    }
    const jobs = await res.json();

    if (!jobs || jobs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="text-center loading-text">No jobs found in database.</td></tr>';
      return;
    }

    tbody.innerHTML = jobs.map(job => {
      const id = job.id || job.ID;
      const name = job.name || job.Name || id;
      const targetFilter = job.target_filter || job.TargetFilter || [];
      const steps = job.steps || job.Steps || [];
      const options = job.options || job.Options || [];

      const filtersHtml = targetFilter.map(t => `<span class="badge badge-tag">${escapeHTML(t)}</span>`).join('') || '<span class="text-muted">None</span>';
      const stepsHtml = steps.map(s => `<span class="badge badge-step">${escapeHTML(s.type || s.Type)}</span>`).join('') || '<span class="text-muted">None</span>';
      const optionsCount = options.length;
      const optionsBadge = optionsCount > 0 ? `<span class="badge badge-step" style="background: rgba(139, 92, 246, 0.2); color: #a78bfa;">⚙️ ${optionsCount} Option(s)</span>` : '';

      return `
        <tr>
          <td>
            <strong>${escapeHTML(name)}</strong> ${optionsBadge}
            <div style="font-size: 0.75rem; color: var(--text-muted); font-family: var(--font-mono);">${id}</div>
          </td>
          <td>${filtersHtml}</td>
          <td>${stepsHtml}</td>
          <td class="text-right">
            <button class="btn btn-primary btn-xs" onclick="runJob('${id}')">
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

// 2. Fetch & Render Schedules
async function loadSchedules() {
  const tbody = document.getElementById('schedules-table-body');
  try {
    const res = await fetch('/api/v1/schedules');
    if (!res.ok) {
      if (res.status === 401) { window.location.href = '/login.html'; return; }
      throw new Error('Failed to fetch schedules');
    }
    const schedules = await res.json();

    if (!schedules || schedules.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" class="text-center loading-text">No recurring schedules active.</td></tr>';
      return;
    }

    tbody.innerHTML = schedules.map(s => {
      const id = s.id || s.ID;
      const jobID = s.job_id || s.JobID;
      const cronExpr = s.cron_expression || s.CronExpression;
      const timezone = s.timezone || s.Timezone || 'UTC';
      const enabled = s.enabled !== undefined ? s.enabled : s.Enabled;

      const statusBadge = enabled 
        ? '<span class="badge badge-status succeeded">Active</span>' 
        : '<span class="badge badge-status failed">Disabled</span>';

      return `
        <tr>
          <td><span class="badge badge-mono">${id}</span></td>
          <td><strong>${escapeHTML(jobID)}</strong></td>
          <td><code style="color: #60a5fa; font-family: var(--font-mono);">${escapeHTML(cronExpr)}</code></td>
          <td>${escapeHTML(timezone)}</td>
          <td>${statusBadge}</td>
          <td class="text-right">
            <button class="btn btn-secondary btn-xs" style="color: #ef4444; border-color: rgba(239, 68, 68, 0.3);" onclick="deleteSchedule('${id}')">
              🗑️ Delete
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="6" class="text-center loading-text" style="color: var(--danger);">Error loading schedules: ${err.message}</td></tr>`;
  }
}

// Prompt to Add a New Schedule
async function createSchedulePrompt() {
  const jobID = prompt("Enter Job ID to schedule:");
  if (!jobID) return;

  const cronExpr = prompt("Enter Cron Expression (e.g. '0 0 2 * * *' for 2 AM daily, '0 */30 * * * *' for every 30m):", "0 0 2 * * *");
  if (!cronExpr) return;

  const timezone = prompt("Enter Timezone (e.g. 'UTC', 'Asia/Kolkata', 'America/New_York'):", "UTC");

  try {
    const res = await fetch('/api/v1/schedules', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        job_id: jobID.trim(),
        cron_expression: cronExpr.trim(),
        timezone: (timezone || 'UTC').trim()
      })
    });

    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to create schedule');
    }

    alert('⏰ Schedule created and registered with Cron Engine successfully!');
    loadSchedules();
  } catch (err) {
    alert(`❌ Failed to create schedule: ${err.message}`);
  }
}

// Delete Schedule
async function deleteSchedule(scheduleID) {
  if (!confirm(`Are you sure you want to delete schedule ${scheduleID}?`)) return;

  try {
    const res = await fetch(`/api/v1/schedules/${encodeURIComponent(scheduleID)}`, {
      method: 'DELETE'
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to delete schedule');
    }
    loadSchedules();
  } catch (err) {
    alert(`❌ Failed to delete schedule: ${err.message}`);
  }
}

// 3. Trigger Execution with Runtime Options
async function runJob(jobID) {
  let optionValues = {};
  try {
    const jobRes = await fetch(`/api/v1/jobs/${encodeURIComponent(jobID)}`);
    if (jobRes.ok) {
      const job = await jobRes.json();
      const options = job.options || job.Options || [];
      if (options.length > 0) {
        for (const opt of options) {
          const name = opt.name || opt.Name;
          const desc = opt.description || opt.Description || '';
          const req = opt.required !== undefined ? opt.required : opt.Required;
          const defVal = opt.default_value !== undefined ? opt.default_value : opt.DefaultValue || '';
          const choices = opt.choices || opt.Choices || [];

          const promptMsg = `Job Option: ${name}\n${desc ? desc + '\n' : ''}${req ? '(Required) ' : ''}Default: '${defVal}'${choices.length ? '\nChoices: ' + choices.join(', ') : ''}`;
          const input = prompt(promptMsg, defVal);
          if (input === null) return; // User cancelled
          if (req && !input && !defVal) {
            alert(`Option '${name}' is required.`);
            return;
          }
          if (input !== '') {
            optionValues[name] = input;
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

// 4. Fetch & Render Executions
async function loadExecutions() {
  const tbody = document.getElementById('executions-table-body');
  try {
    const res = await fetch('/api/v1/executions');
    if (!res.ok) {
      if (res.status === 401) { window.location.href = '/login.html'; return; }
      throw new Error('Failed to fetch executions');
    }
    const executions = await res.json();

    if (!executions || executions.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" class="text-center loading-text">No executions recorded yet.</td></tr>';
      return;
    }

    tbody.innerHTML = executions.map(ex => {
      const id = ex.id || ex.ID;
      const jobID = ex.job_id || ex.JobID;
      const status = ex.status || ex.Status || 'queued';
      const targetsTotal = ex.targets_total !== undefined ? ex.targets_total : (ex.TargetsTotal || 1);
      const startedAt = ex.started_at || ex.StartedAt;

      const statusClass = status.toLowerCase();
      const started = startedAt ? new Date(startedAt).toLocaleString() : '-';

      return `
        <tr>
          <td><span class="badge badge-mono">${id}</span></td>
          <td><strong>${escapeHTML(jobID)}</strong></td>
          <td><span class="badge badge-status ${statusClass}">${escapeHTML(status)}</span></td>
          <td>${targetsTotal} node(s)</td>
          <td style="font-size: 0.8rem; color: var(--text-muted);">${started}</td>
          <td class="text-right">
            <button class="btn btn-secondary btn-xs" onclick="openLogModal('${id}')">
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

// 5. Fetch & Render Targets
async function loadTargets() {
  const tbody = document.getElementById('targets-table-body');
  try {
    const res = await fetch('/api/v1/targets');
    if (!res.ok) {
      if (res.status === 401) { window.location.href = '/login.html'; return; }
      throw new Error('Failed to fetch targets');
    }
    const targets = await res.json();

    if (!targets || targets.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" class="text-center loading-text">No target nodes registered.</td></tr>';
      return;
    }

    tbody.innerHTML = targets.map(t => {
      const id = t.id || t.ID;
      const host = t.host || t.Host;
      const port = t.port || t.Port || '22';
      const user = t.user || t.User;
      const credID = t.credential_id || t.CredentialID;
      const tagsList = t.tags || t.Tags || [];

      const tagsHtml = tagsList.map(tag => `<span class="badge badge-tag">${escapeHTML(tag)}</span>`).join('') || '<span class="text-muted">None</span>';
      const credRef = credID 
        ? `<span class="badge badge-mono" style="background: rgba(16, 185, 129, 0.15); color: #10b981;">🔑 ${escapeHTML(credID)}</span>`
        : `<span class="text-muted" style="font-size: 0.8rem;">Local File Path</span>`;

      return `
        <tr>
          <td><span class="badge badge-mono">${id}</span></td>
          <td><strong>${escapeHTML(host)}</strong></td>
          <td>${port}</td>
          <td>${escapeHTML(user)}</td>
          <td>${credRef}</td>
          <td>${tagsHtml}</td>
          <td class="text-right">
            <button class="btn btn-secondary btn-xs" style="color: #ef4444; border-color: rgba(239, 68, 68, 0.3);" onclick="deleteTarget('${id}')">
              🗑️ Delete
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="7" class="text-center loading-text" style="color: var(--danger);">Error loading target nodes: ${err.message}</td></tr>`;
  }
}

// Prompt to Add a New Target Server
async function createTargetPrompt() {
  const host = prompt("Enter Server Host IP or Hostname (e.g. '192.168.1.50' or 'web-01.local'):");
  if (!host) return;

  const user = prompt("Enter SSH Username (e.g. 'ubuntu', 'root', 'keerthan'):", "ubuntu");
  if (!user) return;

  const port = prompt("Enter SSH Port:", "22");
  if (!port) return;

  const tagsInput = prompt("Enter Tags (comma-separated, e.g. 'web-server, production'):", "web-server");
  const tags = tagsInput ? tagsInput.split(',').map(t => t.trim()).filter(t => t.length > 0) : [];

  const credID = prompt("Enter Credential ID from Key Storage (leave blank for local SSH key):");

  try {
    const res = await fetch('/api/v1/targets', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        host: host.trim(),
        user: user.trim(),
        port: port.trim(),
        tags: tags,
        credential_id: credID ? credID.trim() : ""
      })
    });

    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to add target server');
    }

    alert('🖥️ Target node added to inventory!');
    loadTargets();
  } catch (err) {
    alert(`❌ Failed to add target server: ${err.message}`);
  }
}

// Delete Target Server Node
async function deleteTarget(targetID) {
  if (!confirm(`Are you sure you want to delete target node ${targetID}?`)) return;

  try {
    const res = await fetch(`/api/v1/targets/${encodeURIComponent(targetID)}`, {
      method: 'DELETE'
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to delete target server');
    }
    loadTargets();
  } catch (err) {
    alert(`❌ Failed to delete target server: ${err.message}`);
  }
}

// 6. Fetch & Render Credentials
async function loadCredentials() {
  const tbody = document.getElementById('credentials-table-body');
  try {
    const res = await fetch('/api/v1/credentials');
    if (!res.ok) {
      if (res.status === 401) { window.location.href = '/login.html'; return; }
      throw new Error('Failed to fetch credentials');
    }
    const creds = await res.json();

    if (!creds || creds.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="text-center loading-text">No encrypted credentials stored.</td></tr>';
      return;
    }

    tbody.innerHTML = creds.map(c => {
      const id = c.id || c.ID;
      const name = c.name || c.Name;
      const type = c.type || c.Type || 'ssh_key';
      const createdAt = c.created_at || c.CreatedAt;

      const created = createdAt ? new Date(createdAt).toLocaleString() : '-';
      const typeBadge = `<span class="badge badge-step" style="background: rgba(59, 130, 246, 0.2); color: #60a5fa;">${escapeHTML(type.toUpperCase())}</span>`;

      return `
        <tr>
          <td>
            <strong>${escapeHTML(name)}</strong>
            <div style="font-size: 0.75rem; color: var(--text-muted); font-family: var(--font-mono);">${id}</div>
          </td>
          <td>${typeBadge}</td>
          <td style="font-size: 0.8rem; color: var(--text-muted);">${created}</td>
          <td class="text-right">
            <button class="btn btn-secondary btn-xs" style="color: #ef4444; border-color: rgba(239, 68, 68, 0.3);" onclick="deleteCredential('${id}')">
              🗑️ Delete
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="4" class="text-center loading-text" style="color: var(--danger);">Error loading credentials: ${err.message}</td></tr>`;
  }
}

async function createCredentialPrompt() {
  const name = prompt("Enter Credential Name (e.g. 'Prod Server SSH Key'):");
  if (!name) return;

  const type = prompt("Enter Credential Type ('ssh_key', 'password', 'token'):", "ssh_key");
  if (!type) return;

  const payload = prompt("Paste Credential Content (Private Key PEM string, SSH Password, or API Token):");
  if (!payload) return;

  try {
    const res = await fetch('/api/v1/credentials', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: name.trim(),
        type: type.trim().toLowerCase(),
        payload: payload.trim()
      })
    });

    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to store credential');
    }

    alert('🔒 Secret encrypted via AES-256-GCM and stored securely!');
    loadCredentials();
  } catch (err) {
    alert(`❌ Failed to create credential: ${err.message}`);
  }
}

async function deleteCredential(credID) {
  if (!confirm(`Are you sure you want to delete credential ${credID}?`)) return;

  try {
    const res = await fetch(`/api/v1/credentials/${encodeURIComponent(credID)}`, {
      method: 'DELETE'
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to delete credential');
    }
    loadCredentials();
  } catch (err) {
    alert(`❌ Failed to delete credential: ${err.message}`);
  }
}

// 7. Terminal Log Console Modal
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
      const timestamp = entry.timestamp || entry.Timestamp;
      const targetID = entry.target_id || entry.TargetID || 'node';
      const msg = entry.message || entry.Message || '';

      const time = timestamp ? new Date(timestamp).toLocaleTimeString() : '-';
      return `[${time}] [${targetID}] ${msg}`;
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
