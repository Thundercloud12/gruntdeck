let currentExecutionID = null;
let logPollInterval = null;
let currentProjectID = "";
let currentSelectedJobID = null;
let currentModalFollowMode = "Nodes";
let schedulesCache = [];
let targetsCache = [];
let credentialsCache = [];

// Line-icon set (matches the drawn icons used in index.html; no emoji in the UI)
const ICONS = {
  folder: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"/>',
  eye: '<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/>',
  pencil: '<path d="M4 20l4-1 11-11-3-3-11 11z"/><path d="M14 6l3 3"/>',
  trash: '<path d="M4 7h16"/><path d="M6 7v13a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1V7"/><path d="M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/><path d="M10 11v6"/><path d="M14 11v6"/>',
  play: '<path d="M6 4l14 8-14 8z"/>',
  logs: '<path d="M4 5h16v14H4z"/><path d="M8 9l3 3-3 3"/><path d="M13 15h4"/>',
  key: '<circle cx="8" cy="12" r="4"/><path d="M11 12h10"/><path d="M17 12v4"/><path d="M21 12v3"/>',
  server: '<rect x="3" y="4" width="18" height="6" rx="1"/><rect x="3" y="14" width="18" height="6" rx="1"/><path d="M7 7h.01"/><path d="M7 17h.01"/>',
  gear: '<circle cx="12" cy="12" r="3"/><path d="M12 3v2M12 19v2M4.2 4.2l1.4 1.4M18.4 18.4l1.4 1.4M3 12h2M19 12h2M4.2 19.8l1.4-1.4M18.4 5.6l1.4-1.4"/>',
};

function icon(name, extraClass) {
  const body = ICONS[name] || '';
  return `<svg class="icon${extraClass ? ' ' + extraClass : ''}" viewBox="0 0 24 24">${body}</svg>`;
}

document.addEventListener('DOMContentLoaded', async () => {
  await checkUserSession();
  setupNavigation();
  await loadProjects();
  startTelemetryStrip();
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
    const userBadgeName = document.getElementById('user-badge-name');
    if (userBadgeName && user.username) {
      userBadgeName.textContent = user.username;
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

      if (targetId === 'projects-tab') loadProjects();
      if (targetId === 'jobs-tab') loadJobs();
      if (targetId === 'schedules-tab') loadSchedules();
      if (targetId === 'executions-tab') loadExecutions();
      if (targetId === 'targets-tab') loadTargets();
      if (targetId === 'credentials-tab') loadCredentials();
      loadTelemetryStrip();
    });
  });
}

// Project Selection Handler
function onSelectHeaderProject(projectID) {
  currentProjectID = projectID;
  const jobsTitle = document.getElementById('jobs-section-title');
  if (jobsTitle) {
    jobsTitle.textContent = projectID ? `Automation Jobs (${projectID})` : 'Automation Jobs (All Projects)';
  }

  // Reload current active tab
  const activeTab = document.querySelector('.nav-tab.active');
  if (activeTab) {
    const targetId = activeTab.getAttribute('data-tab');
    if (targetId === 'projects-tab') loadProjects();
    if (targetId === 'jobs-tab') loadJobs();
    if (targetId === 'schedules-tab') loadSchedules();
    if (targetId === 'executions-tab') loadExecutions();
    if (targetId === 'targets-tab') loadTargets();
  }
  loadTelemetryStrip();
}

// ---------- Telemetry Strip (pinned across every tab) ----------
let telemetryPollInterval = null;
let telemetryPrevValues = { targets: null, jobs: null, inflight: null };

function startTelemetryStrip() {
  loadTelemetryStrip();
  telemetryPollInterval = setInterval(() => {
    if (!document.hidden) loadTelemetryStrip();
  }, 5000);
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) loadTelemetryStrip();
  });
}

function flashTelemetryValue(elem, prevVal, newVal) {
  if (!elem) return;
  elem.textContent = newVal;
  if (prevVal !== null && prevVal !== newVal) {
    elem.classList.remove('flash');
    void elem.offsetWidth; // restart the CSS transition
    elem.classList.add('flash');
    setTimeout(() => elem.classList.remove('flash'), 600);
  }
}

async function loadTelemetryStrip() {
  const tilesElem = document.getElementById('telemetry-fleet-tiles');
  const fleetValueElem = document.getElementById('telemetry-fleet-value');
  const jobsValueElem = document.getElementById('telemetry-jobs-value');
  const inflightValueElem = document.getElementById('telemetry-inflight-value');
  const flightRowElem = document.getElementById('telemetry-flight-row');
  if (!tilesElem) return;

  const projQuery = currentProjectID ? `?project_id=${encodeURIComponent(currentProjectID)}` : '';

  try {
    const [targetsRes, jobsRes, execsRes] = await Promise.all([
      fetch(`/api/v1/targets${projQuery}`),
      fetch(`/api/v1/jobs${projQuery}`),
      fetch(`/api/v1/executions${projQuery}`)
    ]);

    if (targetsRes.status === 401 || jobsRes.status === 401 || execsRes.status === 401) {
      window.location.href = '/login.html';
      return;
    }

    const targets = targetsRes.ok ? await targetsRes.json() : [];
    const jobs = jobsRes.ok ? await jobsRes.json() : [];
    const executions = execsRes.ok ? await execsRes.json() : [];

    const targetCount = (targets || []).length;
    const jobCount = (jobs || []).length;
    const inFlight = (executions || []).filter(e => {
      const status = (e.status || e.Status || '').toLowerCase();
      return status === 'running' || status === 'queued';
    });

    // Fleet tiles: one lit tile per registered target (population, not a health check —
    // Gruntdeck does not track per-target liveness, so this never claims more than it knows).
    tilesElem.innerHTML = Array.from({ length: Math.min(targetCount, 40) })
      .map(() => `<div class="fleet-tile up"></div>`).join('');
    flashTelemetryValue(fleetValueElem, telemetryPrevValues.targets, `${targetCount} registered`);
    flashTelemetryValue(jobsValueElem, telemetryPrevValues.jobs, `${jobCount} defined`);
    flashTelemetryValue(inflightValueElem, telemetryPrevValues.inflight, `${inFlight.length} running`);

    telemetryPrevValues = { targets: `${targetCount} registered`, jobs: `${jobCount} defined`, inflight: `${inFlight.length} running` };

    if (inFlight.length === 0) {
      flightRowElem.innerHTML = '<span class="telemetry-empty">No executions in flight</span>';
    } else {
      flightRowElem.innerHTML = inFlight.map(e => {
        const jobID = e.job_id || e.JobID;
        const total = e.targets_total !== undefined ? e.targets_total : (e.TargetsTotal || 0);
        const done = (e.targets_succeeded !== undefined ? e.targets_succeeded : (e.TargetsSucceeded || 0))
          + (e.targets_failed !== undefined ? e.targets_failed : (e.TargetsFailed || 0));
        const pct = total > 0 ? Math.round((done / total) * 100) : 0;
        return `
          <span class="flight-row">
            <span class="flight-job">${escapeHTML(jobID)}</span>
            <span class="flight-bar"><span class="flight-bar-fill" style="width:${pct}%"></span></span>
            <span>${pct}%</span>
          </span>
        `;
      }).join('');
    }
  } catch (err) {
    console.warn('Telemetry strip update failed', err);
  }
}

// 1. Fetch & Render Projects (Rundeck Home)
async function loadProjects() {
  const tbody = document.getElementById('projects-table-body');
  const projectSelect = document.getElementById('header-project-select');

  try {
    const res = await fetch('/api/v1/projects');
    if (!res.ok) {
      if (res.status === 401) { window.location.href = '/login.html'; return; }
      throw new Error('Failed to fetch projects');
    }
    const projects = await res.json();

    // Update Header Dropdown
    if (projectSelect) {
      let optionsHtml = '<option value="">(All Projects)</option>';
      (projects || []).forEach(p => {
        const id = p.id || p.ID;
        const name = p.name || p.Name;
        const isSel = id === currentProjectID ? 'selected' : '';
        optionsHtml += `<option value="${escapeHTML(id)}" ${isSel}>${escapeHTML(name)}</option>`;
      });
      projectSelect.innerHTML = optionsHtml;
    }

    if (!projects || projects.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="text-center loading-text">No projects created yet.</td></tr>';
      return;
    }

    tbody.innerHTML = projects.map(p => {
      const id = p.id || p.ID;
      const name = p.name || p.Name;
      const desc = p.description || p.Description || 'No description';
      const createdAt = p.created_at || p.CreatedAt;
      const created = createdAt ? new Date(createdAt).toLocaleDateString() : '-';

      const isCurrent = id === currentProjectID;
      const activeBadge = isCurrent 
        ? `<span class="badge badge-status succeeded">Active Workspace</span>`
        : '';

      const deleteBtn = id === 'default'
        ? ''
        : `<button class="btn btn-secondary btn-xs btn-danger-ghost" onclick="deleteProject('${id}')">${icon('trash')} Delete</button>`;

      return `
        <tr>
          <td>
            <strong>${escapeHTML(name)}</strong> ${activeBadge}
            <div style="font-size: 0.75rem; color: var(--text-muted); font-family: var(--font-mono);">${id}</div>
          </td>
          <td>${escapeHTML(desc)}</td>
          <td style="font-size: 0.8rem; color: var(--text-muted);">${created}</td>
          <td class="text-right">
            <button class="btn btn-primary btn-xs" onclick="selectProject('${id}')">
              ${icon('folder')} Select Project
            </button>
            ${deleteBtn}
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    if (tbody) tbody.innerHTML = `<tr><td colspan="4" class="text-center loading-text" style="color: var(--danger);">Error loading projects: ${err.message}</td></tr>`;
  }
}

function selectProject(id) {
  const projectSelect = document.getElementById('header-project-select');
  if (projectSelect) {
    projectSelect.value = id;
    onSelectHeaderProject(id);
  }
  document.querySelector('[data-tab="jobs-tab"]').click();
}

async function createProjectPrompt() {
  const name = prompt("Enter Project Name (e.g., 'CICD PIPELINE', 'FetchStockData'):");
  if (!name) return;

  const description = prompt("Enter Project Description (optional):", "Automation project workspace");

  try {
    const res = await fetch('/api/v1/projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: name.trim(),
        description: (description || '').trim()
      })
    });

    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to create project');
    }

    const newProject = await res.json();
    alert(`📁 Project '${newProject.name}' created successfully!`);
    await loadProjects();
    selectProject(newProject.id);
  } catch (err) {
    alert(`❌ Failed to create project: ${err.message}`);
  }
}

async function deleteProject(projectID) {
  if (projectID === 'default') {
    alert('Cannot delete default project.');
    return;
  }
  if (!confirm(`Are you sure you want to delete project '${projectID}' and all its scoped resources?`)) return;

  try {
    const res = await fetch(`/api/v1/projects/${encodeURIComponent(projectID)}`, {
      method: 'DELETE'
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to delete project');
    }

    if (currentProjectID === projectID) {
      currentProjectID = '';
    }
    await loadProjects();
  } catch (err) {
    alert(`❌ Failed to delete project: ${err.message}`);
  }
}

// 2. Fetch & Render Jobs List
async function loadJobs() {
  const tbody = document.getElementById('jobs-table-body');
  try {
    const url = currentProjectID ? `/api/v1/jobs?project_id=${encodeURIComponent(currentProjectID)}` : '/api/v1/jobs';
    const res = await fetch(url);
    if (!res.ok) {
      if (res.status === 401) { window.location.href = '/login.html'; return; }
      throw new Error('Failed to fetch jobs');
    }
    const jobs = await res.json();

    if (!jobs || jobs.length === 0) {
      tbody.innerHTML = `<tr><td colspan="4" class="text-center loading-text">No jobs found ${currentProjectID ? `in project '${currentProjectID}'` : 'in database'}.</td></tr>`;
      return;
    }

    tbody.innerHTML = jobs.map(job => {
      const id = job.id || job.ID;
      const name = job.name || job.Name || id;
      const proj = job.project_id || job.ProjectID || 'default';
      const targetFilter = job.target_filter || job.TargetFilter || [];
      const steps = job.steps || job.Steps || [];
      const options = job.options || job.Options || [];

      const filtersHtml = targetFilter.map(t => `<span class="badge badge-tag">${escapeHTML(t)}</span>`).join('') || '<span class="text-muted">None</span>';
      const stepsHtml = steps.map(s => `<span class="badge badge-step">${escapeHTML(s.type || s.Type)}</span>`).join('') || '<span class="text-muted">None</span>';
      const optionsCount = options.length;
      const optionsBadge = optionsCount > 0 ? `<span class="badge badge-role">${icon('gear')} ${optionsCount} Option(s)</span>` : '';
      const projBadge = `<span class="badge badge-mono">${icon('folder')} ${escapeHTML(proj)}</span>`;

      return `
        <tr>
          <td>
            <a href="javascript:void(0)" onclick="openJobDetails('${id}')" style="color: var(--text-main); text-decoration: none; font-weight: 600;">
              ${escapeHTML(name)}
            </a> ${optionsBadge} ${projBadge}
            <div style="font-size: 0.75rem; color: var(--text-muted); font-family: var(--font-mono); cursor: pointer;" onclick="openJobDetails('${id}')">${id}</div>
          </td>
          <td>${filtersHtml}</td>
          <td>${stepsHtml}</td>
          <td class="text-right">
            <button class="btn btn-secondary btn-xs" onclick="openJobDetails('${id}')" style="margin-right: 0.4rem;">
              ${icon('eye')} View Job
            </button>
            <button class="btn btn-secondary btn-xs" onclick="openJobEditor('${id}')" style="margin-right: 0.4rem;">
              ${icon('pencil')} Edit
            </button>
            <button class="btn btn-primary btn-xs" onclick="runJob('${id}')" style="margin-right: 0.4rem;">
              ${icon('play')} Run Job
            </button>
            <button class="btn btn-secondary btn-xs btn-danger-ghost" onclick="deleteJob('${id}')">
              ${icon('trash')} Delete
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="4" class="text-center loading-text" style="color: var(--danger);">Error loading jobs: ${err.message}</td></tr>`;
  }
}

// 2b. Job Editor Modal (Create / Edit)
let jobEditorEditingID = null;

async function openJobEditor(jobID) {
  jobEditorEditingID = jobID || null;
  document.getElementById('job-editor-title').textContent = jobID ? 'Edit Job' : 'New Job';
  document.getElementById('job-editor-name').value = '';
  document.getElementById('job-editor-target-filter').value = '';
  document.getElementById('job-editor-steps').value = '[]';
  document.getElementById('job-editor-options').value = '[]';

  if (jobID) {
    try {
      const res = await fetch(`/api/v1/jobs/${encodeURIComponent(jobID)}`);
      if (res.ok) {
        const job = await res.json();
        document.getElementById('job-editor-name').value = job.name || job.Name || '';
        document.getElementById('job-editor-target-filter').value = (job.target_filter || job.TargetFilter || []).join(', ');
        document.getElementById('job-editor-steps').value = JSON.stringify(job.steps || job.Steps || [], null, 2);
        document.getElementById('job-editor-options').value = JSON.stringify(job.options || job.Options || [], null, 2);
      }
    } catch (err) {
      alert(`❌ Failed to load job for editing: ${err.message}`);
      return;
    }
  }

  document.getElementById('job-editor-modal').classList.remove('hidden');
}

function closeJobEditor() {
  document.getElementById('job-editor-modal').classList.add('hidden');
  jobEditorEditingID = null;
}

async function submitJobEditor() {
  const name = document.getElementById('job-editor-name').value.trim();
  if (!name) {
    alert('❌ Job name is required.');
    return;
  }

  const targetFilter = document.getElementById('job-editor-target-filter').value
    .split(',').map(t => t.trim()).filter(t => t.length > 0);

  let steps, options;
  try {
    steps = JSON.parse(document.getElementById('job-editor-steps').value || '[]');
    if (!Array.isArray(steps)) throw new Error('steps must be a JSON array');
  } catch (err) {
    alert(`❌ Invalid steps JSON: ${err.message}`);
    return;
  }
  try {
    options = JSON.parse(document.getElementById('job-editor-options').value || '[]');
    if (!Array.isArray(options)) throw new Error('options must be a JSON array');
  } catch (err) {
    alert(`❌ Invalid options JSON: ${err.message}`);
    return;
  }

  const payload = {
    name,
    project_id: currentProjectID || 'default',
    target_filter: targetFilter,
    steps,
    options
  };

  try {
    const url = jobEditorEditingID ? `/api/v1/jobs/${encodeURIComponent(jobEditorEditingID)}` : '/api/v1/jobs';
    const method = jobEditorEditingID ? 'PUT' : 'POST';
    const res = await fetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to save job');
    }

    closeJobEditor();
    await loadJobs();
  } catch (err) {
    alert(`❌ Failed to save job: ${err.message}`);
  }
}

async function deleteJob(jobID) {
  if (!jobID) return;
  if (!confirm(`Are you sure you want to delete job ${jobID}? This cannot be undone.`)) return;

  try {
    const res = await fetch(`/api/v1/jobs/${encodeURIComponent(jobID)}`, { method: 'DELETE' });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to delete job');
    }

    if (currentSelectedJobID === jobID) {
      backToJobsList();
    }
    await loadJobs();
  } catch (err) {
    alert(`❌ Failed to delete job: ${err.message}`);
  }
}

// 3. Job Details / Show Screen
async function openJobDetails(jobID) {
  currentSelectedJobID = jobID;

  // Switch tab visibility
  document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
  document.querySelectorAll('.nav-tab').forEach(t => t.classList.remove('active'));
  const detailsTab = document.getElementById('job-details-tab');
  if (detailsTab) detailsTab.classList.add('active');

  // Load Job Metadata
  try {
    const jobRes = await fetch(`/api/v1/jobs/${encodeURIComponent(jobID)}`);
    if (jobRes.ok) {
      const job = await jobRes.json();
      document.getElementById('job-detail-name').textContent = job.name || job.Name || jobID;
      document.getElementById('job-detail-id').textContent = `id: ${jobID}`;
    }
  } catch (err) {
    console.error("Failed to load job details", err);
  }

  // Load Stats & Activity
  await loadJobStats(jobID);
  await loadJobActivity(jobID);
}

function backToJobsList() {
  currentSelectedJobID = null;
  document.querySelector('[data-tab="jobs-tab"]').click();
}

function switchJobSubtab(subpanelID) {
  document.querySelectorAll('.subnav-tab').forEach(t => t.classList.remove('active'));
  document.querySelectorAll('.subpanel-content').forEach(p => p.classList.remove('active'));

  const btn = document.querySelector(`.subnav-tab[data-subtab="${subpanelID}"]`);
  if (btn) btn.classList.add('active');

  const panel = document.getElementById(subpanelID);
  if (panel) panel.classList.add('active');
}

async function loadJobStats(jobID) {
  try {
    const res = await fetch(`/api/v1/jobs/${encodeURIComponent(jobID)}/stats`);
    if (!res.ok) return;
    const stats = await res.json();

    document.getElementById('stat-total-executions').textContent = stats.total_executions || 0;
    document.getElementById('stat-success-rate').textContent = `${stats.success_rate || 100}%`;
    document.getElementById('stat-avg-duration').textContent = `${stats.avg_duration_seconds || 0}s`;
  } catch (err) {
    console.error("Failed to fetch job stats", err);
  }
}

async function loadJobActivity(jobID) {
  const tbody = document.getElementById('job-activity-table-body');
  try {
    const res = await fetch(`/api/v1/jobs/${encodeURIComponent(jobID)}/executions`);
    if (!res.ok) throw new Error('Failed to fetch job executions');
    const execs = await res.json();

    if (!execs || execs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" class="text-center loading-text">No activity history for this job yet.</td></tr>';
      return;
    }

    tbody.innerHTML = execs.map(ex => {
      const id = ex.id || ex.ID;
      const status = ex.status || ex.Status || 'queued';
      const startedAt = ex.started_at || ex.StartedAt;
      const completedAt = ex.completed_at || ex.CompletedAt;

      const started = startedAt ? new Date(startedAt).toLocaleString() : '-';
      let durationStr = 'running';
      if (startedAt && completedAt) {
        const durSec = Math.round((new Date(completedAt) - new Date(startedAt)) / 1000);
        durationStr = `${durSec} seconds`;
      }

      const isOk = status.toLowerCase() === 'succeeded' || status.toLowerCase() === 'completed';
      const statusBadge = isOk 
        ? '<span class="badge badge-status succeeded">1 ok</span>'
        : `<span class="badge badge-status failed">${escapeHTML(status)}</span>`;

      return `
        <tr>
          <td>${statusBadge}</td>
          <td style="font-size: 0.8rem; color: var(--text-muted);">${started}</td>
          <td>${durationStr}</td>
          <td><span class="badge badge-mono">by admin</span></td>
          <td><strong>${escapeHTML(ex.job_id || ex.JobID)}</strong></td>
          <td class="text-right">
            <button class="btn btn-secondary btn-xs" onclick="openLogModal('${id}')">
              #${id.substring(0, 8)}
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="6" class="text-center loading-text" style="color: var(--danger);">Error loading activity: ${err.message}</td></tr>`;
  }
}

function runCurrentJobDetail() {
  if (currentSelectedJobID) {
    runJob(currentSelectedJobID);
  }
}

// 4. Fetch & Render Schedules
async function loadSchedules() {
  const tbody = document.getElementById('schedules-table-body');
  try {
    const url = currentProjectID ? `/api/v1/schedules?project_id=${encodeURIComponent(currentProjectID)}` : '/api/v1/schedules';
    const res = await fetch(url);
    if (!res.ok) {
      if (res.status === 401) { window.location.href = '/login.html'; return; }
      throw new Error('Failed to fetch schedules');
    }
    const schedules = await res.json();
    schedulesCache = schedules || [];

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
          <td><code style="color: var(--accent); font-family: var(--font-mono);">${escapeHTML(cronExpr)}</code></td>
          <td>${escapeHTML(timezone)}</td>
          <td>${statusBadge}</td>
          <td class="text-right">
            <button class="btn btn-secondary btn-xs" onclick="openScheduleEditor('${id}')" style="margin-right: 0.4rem;">
              ${icon('pencil')} Edit
            </button>
            <button class="btn btn-secondary btn-xs btn-danger-ghost" onclick="deleteSchedule('${id}')">
              ${icon('trash')} Delete
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="6" class="text-center loading-text" style="color: var(--danger);">Error loading schedules: ${err.message}</td></tr>`;
  }
}

// Schedule Editor Modal (Create / Edit)
let scheduleEditorEditingID = null;

function openScheduleEditor(scheduleID) {
  scheduleEditorEditingID = scheduleID || null;
  document.getElementById('schedule-editor-title').textContent = scheduleID ? 'Edit Schedule' : 'New Schedule';
  document.getElementById('schedule-editor-job-id').value = '';
  document.getElementById('schedule-editor-cron').value = '0 0 2 * * *';
  document.getElementById('schedule-editor-timezone').value = 'UTC';
  document.getElementById('schedule-editor-enabled').checked = true;

  if (scheduleID) {
    const sched = schedulesCache.find(s => (s.id || s.ID) === scheduleID);
    if (sched) {
      document.getElementById('schedule-editor-job-id').value = sched.job_id || sched.JobID || '';
      document.getElementById('schedule-editor-cron').value = sched.cron_expression || sched.CronExpression || '';
      document.getElementById('schedule-editor-timezone').value = sched.timezone || sched.Timezone || 'UTC';
      const enabled = sched.enabled !== undefined ? sched.enabled : sched.Enabled;
      document.getElementById('schedule-editor-enabled').checked = enabled !== false;
    }
  }

  document.getElementById('schedule-editor-modal').classList.remove('hidden');
}

function closeScheduleEditor() {
  document.getElementById('schedule-editor-modal').classList.add('hidden');
  scheduleEditorEditingID = null;
}

async function submitScheduleEditor() {
  const jobID = document.getElementById('schedule-editor-job-id').value.trim();
  const cronExpr = document.getElementById('schedule-editor-cron').value.trim();
  const timezone = document.getElementById('schedule-editor-timezone').value.trim() || 'UTC';
  const enabled = document.getElementById('schedule-editor-enabled').checked;

  if (!jobID || !cronExpr) {
    alert('❌ Job ID and Cron Expression are required.');
    return;
  }

  const payload = {
    project_id: currentProjectID || 'default',
    job_id: jobID,
    cron_expression: cronExpr,
    timezone,
    enabled
  };

  try {
    const url = scheduleEditorEditingID ? `/api/v1/schedules/${encodeURIComponent(scheduleEditorEditingID)}` : '/api/v1/schedules';
    const method = scheduleEditorEditingID ? 'PUT' : 'POST';
    const res = await fetch(url, {
      method,
      headers: {
        'Content-Type': 'application/json',
        'X-Project-ID': currentProjectID || 'default'
      },
      body: JSON.stringify(payload)
    });

    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to save schedule');
    }

    closeScheduleEditor();
    await loadSchedules();
  } catch (err) {
    alert(`❌ Failed to save schedule: ${err.message}`);
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

// 5. Trigger Execution with Runtime Options
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

    // Read selected follow execution mode from Job detail screen if active
    const detailModeSelect = document.getElementById('job-follow-mode-select');
    if (detailModeSelect) {
      currentModalFollowMode = detailModeSelect.value;
      const modalSelect = document.getElementById('modal-follow-mode-select');
      if (modalSelect) modalSelect.value = currentModalFollowMode;
    }

    openLogModal(data.execution_id);
    loadTelemetryStrip();
  } catch (err) {
    alert(`❌ Failed to run job: ${err.message}`);
  }
}

// 6. Fetch & Render Executions
async function loadExecutions() {
  const tbody = document.getElementById('executions-table-body');
  try {
    const url = currentProjectID ? `/api/v1/executions?project_id=${encodeURIComponent(currentProjectID)}` : '/api/v1/executions';
    const res = await fetch(url);
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
              ${icon('logs')} View Logs
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="6" class="text-center loading-text" style="color: var(--danger);">Error loading executions: ${err.message}</td></tr>`;
  }
}

// 7. Fetch & Render Targets
async function loadTargets() {
  const tbody = document.getElementById('targets-table-body');
  try {
    const url = currentProjectID ? `/api/v1/targets?project_id=${encodeURIComponent(currentProjectID)}` : '/api/v1/targets';
    const res = await fetch(url);
    if (!res.ok) {
      if (res.status === 401) { window.location.href = '/login.html'; return; }
      throw new Error('Failed to fetch targets');
    }
    const targets = await res.json();
    targetsCache = targets || [];

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
        ? `<span class="badge badge-credential">${icon('key')} ${escapeHTML(credID)}</span>`
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
            <button class="btn btn-secondary btn-xs" onclick="openTargetEditor('${id}')" style="margin-right: 0.4rem;">
              ${icon('pencil')} Edit
            </button>
            <button class="btn btn-secondary btn-xs btn-danger-ghost" onclick="deleteTarget('${id}')">
              ${icon('trash')} Delete
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="7" class="text-center loading-text" style="color: var(--danger);">Error loading target nodes: ${err.message}</td></tr>`;
  }
}

// Target Editor Modal (Create / Edit)
let targetEditorEditingID = null;

function openTargetEditor(targetID) {
  targetEditorEditingID = targetID || null;
  document.getElementById('target-editor-title').textContent = targetID ? 'Edit Target Node' : 'New Target Node';
  document.getElementById('target-editor-host').value = '';
  document.getElementById('target-editor-user').value = 'ubuntu';
  document.getElementById('target-editor-port').value = '22';
  document.getElementById('target-editor-tags').value = '';
  document.getElementById('target-editor-credential').value = '';

  if (targetID) {
    const target = targetsCache.find(t => (t.id || t.ID) === targetID);
    if (target) {
      document.getElementById('target-editor-host').value = target.host || target.Host || '';
      document.getElementById('target-editor-user').value = target.user || target.User || '';
      document.getElementById('target-editor-port').value = target.port || target.Port || '22';
      document.getElementById('target-editor-tags').value = (target.tags || target.Tags || []).join(', ');
      document.getElementById('target-editor-credential').value = target.credential_id || target.CredentialID || '';
    }
  }

  document.getElementById('target-editor-modal').classList.remove('hidden');
}

function closeTargetEditor() {
  document.getElementById('target-editor-modal').classList.add('hidden');
  targetEditorEditingID = null;
}

async function submitTargetEditor() {
  const host = document.getElementById('target-editor-host').value.trim();
  const user = document.getElementById('target-editor-user').value.trim();
  const port = document.getElementById('target-editor-port').value.trim() || '22';
  const tags = document.getElementById('target-editor-tags').value
    .split(',').map(t => t.trim()).filter(t => t.length > 0);
  const credID = document.getElementById('target-editor-credential').value.trim();

  if (!host || !user) {
    alert('❌ Host and SSH User are required.');
    return;
  }

  const payload = {
    project_id: currentProjectID || 'default',
    host,
    user,
    port,
    tags,
    credential_id: credID
  };

  try {
    const url = targetEditorEditingID ? `/api/v1/targets/${encodeURIComponent(targetEditorEditingID)}` : '/api/v1/targets';
    const method = targetEditorEditingID ? 'PUT' : 'POST';
    const res = await fetch(url, {
      method,
      headers: {
        'Content-Type': 'application/json',
        'X-Project-ID': currentProjectID || 'default'
      },
      body: JSON.stringify(payload)
    });

    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to save target node');
    }

    closeTargetEditor();
    await loadTargets();
  } catch (err) {
    alert(`❌ Failed to save target node: ${err.message}`);
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

// 8. Fetch & Render Credentials
async function loadCredentials() {
  const tbody = document.getElementById('credentials-table-body');
  try {
    const res = await fetch('/api/v1/credentials');
    if (!res.ok) {
      if (res.status === 401) { window.location.href = '/login.html'; return; }
      throw new Error('Failed to fetch credentials');
    }
    const creds = await res.json();
    credentialsCache = creds || [];

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
      const typeBadge = `<span class="badge badge-type">${escapeHTML(type.toUpperCase())}</span>`;

      return `
        <tr>
          <td>
            <strong>${escapeHTML(name)}</strong>
            <div style="font-size: 0.75rem; color: var(--text-muted); font-family: var(--font-mono);">${id}</div>
          </td>
          <td>${typeBadge}</td>
          <td style="font-size: 0.8rem; color: var(--text-muted);">${created}</td>
          <td class="text-right">
            <button class="btn btn-secondary btn-xs" onclick="openCredentialEditor('${id}')" style="margin-right: 0.4rem;">
              ${icon('pencil')} Edit
            </button>
            <button class="btn btn-secondary btn-xs btn-danger-ghost" onclick="deleteCredential('${id}')">
              ${icon('trash')} Delete
            </button>
          </td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="4" class="text-center loading-text" style="color: var(--danger);">Error loading credentials: ${err.message}</td></tr>`;
  }
}

// Credential Editor Modal (Create / Edit)
let credentialEditorEditingID = null;

function openCredentialEditor(credID) {
  credentialEditorEditingID = credID || null;
  document.getElementById('credential-editor-title').textContent = credID ? 'Edit Key / Secret' : 'New Key / Secret';
  document.getElementById('credential-editor-name').value = '';
  document.getElementById('credential-editor-type').value = 'ssh_key';
  document.getElementById('credential-editor-payload').value = '';
  document.getElementById('credential-editor-payload-hint').classList.toggle('hidden', !credID);

  if (credID) {
    const cred = credentialsCache.find(c => (c.id || c.ID) === credID);
    if (cred) {
      document.getElementById('credential-editor-name').value = cred.name || cred.Name || '';
      document.getElementById('credential-editor-type').value = cred.type || cred.Type || 'ssh_key';
    }
  }

  document.getElementById('credential-editor-modal').classList.remove('hidden');
}

function closeCredentialEditor() {
  document.getElementById('credential-editor-modal').classList.add('hidden');
  credentialEditorEditingID = null;
}

async function submitCredentialEditor() {
  const name = document.getElementById('credential-editor-name').value.trim();
  const type = document.getElementById('credential-editor-type').value;
  const payload = document.getElementById('credential-editor-payload').value.trim();

  if (!name) {
    alert('❌ Name is required.');
    return;
  }
  if (!credentialEditorEditingID && !payload) {
    alert('❌ Secret content is required.');
    return;
  }

  try {
    const url = credentialEditorEditingID ? `/api/v1/credentials/${encodeURIComponent(credentialEditorEditingID)}` : '/api/v1/credentials';
    const method = credentialEditorEditingID ? 'PUT' : 'POST';
    const res = await fetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, type, payload })
    });

    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Failed to save credential');
    }

    closeCredentialEditor();
    await loadCredentials();
  } catch (err) {
    alert(`❌ Failed to save credential: ${err.message}`);
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

// 9. Execution Follow Modes & Log Console Modal
function onChangeModalFollowMode(mode) {
  currentModalFollowMode = mode;
  if (currentExecutionID) {
    fetchLogsForModal(currentExecutionID);
  }
}

function openLogModal(executionID) {
  currentExecutionID = executionID;
  document.getElementById('modal-execution-id').innerText = `ID: ${executionID}`;

  const modalSelect = document.getElementById('modal-follow-mode-select');
  if (modalSelect) modalSelect.value = currentModalFollowMode;

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
  const frameElem = document.getElementById('terminal-html-frame');
  const statusElem = document.getElementById('terminal-status');

  try {
    const res = await fetch(`/api/v1/executions/${encodeURIComponent(executionID)}/logs`);
    if (!res.ok) throw new Error('Failed to fetch logs');
    const logs = await res.json();

    if (!logs || logs.length === 0) {
      outputElem.classList.remove('hidden');
      if (frameElem) frameElem.classList.add('hidden');
      outputElem.textContent = '⏳ Waiting for log stream from executor daemon...';
      statusElem.textContent = `Status: Waiting for output (${currentModalFollowMode} Mode)`;
      return;
    }

    statusElem.textContent = `Status: ${logs.length} log entry(ies) recorded [View: ${currentModalFollowMode}]`;

    // 1. HTML View Mode
    if (currentModalFollowMode === "HTML") {
      outputElem.classList.add('hidden');
      if (frameElem) {
        frameElem.classList.remove('hidden');
        const rawHTML = logs.map(l => l.message || l.Message || '').join('\n');
        const fullHTML = `
          <!DOCTYPE html>
          <html>
            <head>
              <style>
                body { font-family: sans-serif; padding: 1rem; color: #1e293b; background: #f8fafc; }
                table { border-collapse: collapse; width: 100%; margin-top: 0.5rem; }
                th, td { border: 1px solid #cbd5e1; padding: 8px 12px; text-align: left; }
                th { background-color: #e2e8f0; }
              </style>
            </head>
            <body>
              ${rawHTML}
            </body>
          </html>
        `;
        frameElem.srcdoc = fullHTML;
      }
      return;
    }

    // Hide HTML iframe for text modes
    if (frameElem) frameElem.classList.add('hidden');
    outputElem.classList.remove('hidden');

    // 2. Nodes View Mode (Grouped by Server Node)
    if (currentModalFollowMode === "Nodes") {
      const nodeLogsMap = {};
      logs.forEach(entry => {
        const targetID = entry.target_id || entry.TargetID || 'default-node';
        if (!nodeLogsMap[targetID]) nodeLogsMap[targetID] = [];
        const timestamp = entry.timestamp || entry.Timestamp;
        const time = timestamp ? new Date(timestamp).toLocaleTimeString() : '-';
        nodeLogsMap[targetID].push(`[${time}] ${entry.message || entry.Message || ''}`);
      });

      let nodesFormatted = '';
      for (const [node, lines] of Object.entries(nodeLogsMap)) {
        nodesFormatted += `── NODE ${node} ──\n`;
        nodesFormatted += lines.join('\n') + '\n\n';
      }
      outputElem.textContent = nodesFormatted.trim();
      outputElem.scrollTop = outputElem.scrollHeight;
      return;
    }

    // 3. Log Output View Mode (Raw Terminal Stream)
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
