const state = {
  token: window.localStorage.getItem("adp.token") || "",
  user: null,
  toastTimer: null,
  refreshTimer: null,
};

const page = document.body.dataset.page || "home";

const elements = {
  clock: byId("clock"),
  sessionState: byId("session-state"),
  refreshPage: byId("refresh-page"),
  toast: byId("toast"),
  loginForm: byId("login-form"),
  loginMessage: byId("login-message"),
  logoutButton: byId("logout-button"),
  metricsGrid: byId("metrics-grid"),
  approvalList: byId("approval-list"),
  auditList: byId("audit-list"),
  userForm: byId("user-form"),
  usersAccessNote: byId("users-access-note"),
  userList: byId("user-list"),
  workerForm: byId("worker-form"),
  workerList: byId("worker-list"),
  jobForm: byId("job-form"),
  jobList: byId("job-list"),
  taskForm: byId("task-form"),
  taskInput: byId("task-input"),
  taskParams: byId("task-params"),
  taskOutput: byId("task-output"),
  taskList: byId("task-list"),
  templateList: byId("template-list"),
};

boot();

function boot() {
  startClock();
  bindCommonEvents();
  updateSessionState();
  renderLoggedOutPlaceholders();

  if (state.token) {
    refreshCurrentPage();
    state.refreshTimer = window.setInterval(refreshCurrentPage, 15000);
  }
}

function bindCommonEvents() {
  elements.refreshPage?.addEventListener("click", () => refreshCurrentPage());
  elements.logoutButton?.addEventListener("click", handleLogout);
  elements.loginForm?.addEventListener("submit", handleLogin);
  elements.userForm?.addEventListener("submit", handleCreateUser);
  elements.workerForm?.addEventListener("submit", handleCreateWorker);
  elements.jobForm?.addEventListener("submit", handleCreateJob);
  elements.taskForm?.addEventListener("submit", handleTaskSubmit);
  elements.approvalList?.addEventListener("click", handleApprovalAction);
}

function startClock() {
  const tick = () => {
    if (elements.clock) {
      elements.clock.textContent = new Date().toLocaleTimeString("zh-CN", { hour12: false });
    }
  };
  tick();
  window.setInterval(tick, 1000);
}

async function handleLogin(event) {
  event.preventDefault();

  const formData = new FormData(elements.loginForm);
  const username = String(formData.get("username") || "").trim();
  const password = String(formData.get("password") || "").trim();

  try {
    const result = await request("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
    state.token = result.token;
    state.user = result.user;
    window.localStorage.setItem("adp.token", state.token);
    updateSessionState();
    if (elements.loginMessage) {
      elements.loginMessage.textContent = "登录成功。";
    }
    showToast("已登录");

    if (!state.refreshTimer) {
      state.refreshTimer = window.setInterval(refreshCurrentPage, 15000);
    }

    if (page === "login") {
      window.location.href = "/";
      return;
    }

    await refreshCurrentPage();
  } catch (error) {
    if (elements.loginMessage) {
      elements.loginMessage.textContent = error.message;
    }
    showToast(error.message);
  }
}

function handleLogout() {
  state.token = "";
  state.user = null;
  window.localStorage.removeItem("adp.token");
  if (state.refreshTimer) {
    window.clearInterval(state.refreshTimer);
    state.refreshTimer = null;
  }
  updateSessionState();
  renderLoggedOutPlaceholders();
  if (elements.loginMessage) {
    elements.loginMessage.textContent = "已退出登录。";
  }
  showToast("已退出登录");
}

async function handleCreateUser(event) {
  event.preventDefault();
  if (!ensureAuthed()) {
    return;
  }

  try {
    const user = await authedRequest("/api/v1/users", {
      method: "POST",
      body: JSON.stringify({
        username: valueOf("new-username"),
        password: valueOf("new-password"),
        role: valueOf("new-role"),
      }),
    });
    elements.userForm.reset();
    byId("new-role").value = "operator";
    showToast(`用户 ${user.username} 已创建`);
    await refreshUsersPage();
  } catch (error) {
    showToast(error.message);
  }
}

async function handleCreateWorker(event) {
  event.preventDefault();
  if (!ensureAuthed()) {
    return;
  }

  try {
    const worker = await authedRequest("/api/v1/workers", {
      method: "POST",
      body: JSON.stringify({
        name: valueOf("worker-name"),
        worker_type: valueOf("worker-type"),
      }),
    });
    elements.workerForm.reset();
    byId("worker-type").value = "shell";
    showToast(`Worker ${worker.name} 已创建`);
    await refreshWorkersPage();
  } catch (error) {
    showToast(error.message);
  }
}

async function handleCreateJob(event) {
  event.preventDefault();
  if (!ensureAuthed()) {
    return;
  }

  try {
    const job = await authedRequest("/api/v1/jobs", {
      method: "POST",
      body: JSON.stringify({
        name: valueOf("job-name"),
        worker_type: valueOf("job-worker-type"),
        command: valueOf("job-command"),
      }),
    });
    elements.jobForm.reset();
    byId("job-worker-type").value = "shell";
    showToast(`Job ${job.id} 已创建`);
    await refreshJobsPage();
  } catch (error) {
    showToast(error.message);
  }
}

async function handleTaskSubmit(event) {
  event.preventDefault();
  if (!ensureAuthed()) {
    return;
  }

  const action = event.submitter?.dataset.action || "parse";
  const input = elements.taskInput?.value.trim();
  if (!input) {
    showToast("先输入任务描述");
    return;
  }

  let parameters;
  try {
    parameters = parseOptionalJSON(elements.taskParams?.value.trim() || "");
  } catch (error) {
    showToast(error.message);
    return;
  }

  try {
    if (action === "parse") {
      const result = await authedRequest("/api/v1/tasks/parse", {
        method: "POST",
        body: JSON.stringify({ input }),
      });
      if (elements.taskOutput) {
        elements.taskOutput.textContent = JSON.stringify(result, null, 2);
      }
      showToast("Task 解析完成");
      return;
    }

    const result = await authedRequest("/api/v1/tasks/run", {
      method: "POST",
      body: JSON.stringify({ input, parameters }),
    });
    if (elements.taskOutput) {
      elements.taskOutput.textContent = JSON.stringify(result, null, 2);
    }
    showToast(result.approval_required ? "Task 已进入审批队列" : "Task 已创建");
    await refreshTasksPage();
  } catch (error) {
    if (elements.taskOutput) {
      elements.taskOutput.textContent = error.message;
    }
    showToast(error.message);
  }
}

async function handleApprovalAction(event) {
  const button = event.target.closest("[data-approval-id]");
  if (!button) {
    return;
  }
  if (!ensureAuthed()) {
    return;
  }

  const approved = button.dataset.decision === "approve";
  try {
    const result = await authedRequest(`/api/v1/approvals/jobs/${button.dataset.approvalId}`, {
      method: "POST",
      body: JSON.stringify({
        approved,
        comment: approved ? "Approved from UI" : "Rejected from UI",
      }),
    });
    showToast(approved ? `已批准 ${result.id}` : `已拒绝 ${result.id}`);
    await refreshCurrentPage();
  } catch (error) {
    showToast(error.message);
  }
}

async function refreshCurrentPage() {
  if (!state.token) {
    renderLoggedOutPlaceholders();
    return;
  }

  try {
    switch (page) {
      case "home":
        await refreshHomePage();
        break;
      case "users":
        await refreshUsersPage();
        break;
      case "workers":
        await refreshWorkersPage();
        break;
      case "jobs":
        await refreshJobsPage();
        break;
      case "tasks":
        await refreshTasksPage();
        break;
      default:
        await refreshSessionOnly();
        break;
    }
  } catch (error) {
    if (error.code === 401) {
      handleLogout();
      return;
    }
    showToast(error.message);
  }
}

async function refreshSessionOnly() {
  const summary = await authedRequest("/api/v1/dashboard/summary");
  state.user = summary.user;
  updateSessionState(summary.current_time);
}

async function refreshHomePage() {
  const summary = await authedRequest("/api/v1/dashboard/summary");
  state.user = summary.user;
  updateSessionState(summary.current_time);
  renderSummaryMetrics(summary);
  renderApprovals(summary.pending_approvals);
  renderAuditLogs(summary.recent_audit_logs);
}

async function refreshUsersPage() {
  const summary = await authedRequest("/api/v1/dashboard/summary");
  state.user = summary.user;
  updateSessionState(summary.current_time);

  if (state.user.role !== "admin") {
    if (elements.usersAccessNote) {
      elements.usersAccessNote.textContent = "当前账户不是管理员，无法查看或创建用户。";
    }
    renderList(elements.userList, [], () => "", "请使用管理员账户登录。");
    return;
  }

  if (elements.usersAccessNote) {
    elements.usersAccessNote.textContent = "当前为管理员，可创建与查看用户。";
  }

  const users = await authedRequest("/api/v1/users");
  renderList(
    elements.userList,
    users,
    (user) => `
      <article class="list-card">
        <div class="list-row">
          <div>
            <h4>${escapeHTML(user.username)}</h4>
            <p>角色：${escapeHTML(user.role)}</p>
          </div>
          <span class="status-pill">${escapeHTML(user.role)}</span>
        </div>
      </article>
    `,
    "暂无用户。"
  );
}

async function refreshWorkersPage() {
  const summary = await authedRequest("/api/v1/dashboard/summary");
  state.user = summary.user;
  updateSessionState(summary.current_time);

  const workers = await authedRequest("/api/v1/workers");
  renderList(
    elements.workerList,
    workers,
    (worker) => `
      <article class="list-card">
        <div class="list-row">
          <div>
            <h4>${escapeHTML(worker.name)}</h4>
            <p>${escapeHTML(worker.id)} / ${escapeHTML(worker.worker_type)}</p>
          </div>
          <span class="status-pill ${statusClass(worker.status)}">${escapeHTML(worker.status)}</span>
        </div>
        <div class="list-meta">
          <span>最近心跳 ${formatTime(worker.last_heartbeat_at)}</span>
          <span>创建于 ${formatTime(worker.created_at)}</span>
        </div>
      </article>
    `,
    "暂无 Worker。"
  );
}

async function refreshJobsPage() {
  const summary = await authedRequest("/api/v1/dashboard/summary");
  state.user = summary.user;
  updateSessionState(summary.current_time);

  const jobs = await authedRequest("/api/v1/jobs?limit=16");
  renderList(
    elements.jobList,
    jobs,
    (job) => `
      <article class="list-card">
        <div class="list-row">
          <div>
            <h4>${escapeHTML(job.name)}</h4>
            <p>${escapeHTML(job.command || "无命令详情")}</p>
          </div>
          <span class="status-pill ${statusClass(job.status)}">${escapeHTML(job.status)}</span>
        </div>
        <div class="list-meta">
          <span>${escapeHTML(job.id)}</span>
          <span>${escapeHTML(job.worker_type)}</span>
          <span>${escapeHTML(job.source_type || "manual_job")}</span>
          <span>${formatTime(job.updated_at)}</span>
        </div>
      </article>
    `,
    "暂无 Job。"
  );
  renderApprovals(summary.pending_approvals);
}

async function refreshTasksPage() {
  const summary = await authedRequest("/api/v1/dashboard/summary");
  state.user = summary.user;
  updateSessionState(summary.current_time);

  const [tasks, templates] = await Promise.all([
    authedRequest("/api/v1/tasks"),
    authedRequest("/api/v1/templates"),
  ]);

  renderList(
    elements.taskList,
    tasks,
    (task) => `
      <article class="list-card">
        <div class="list-row">
          <div>
            <h4>${escapeHTML(task.name)}</h4>
            <p>${escapeHTML(task.command || "无命令详情")}</p>
          </div>
          <span class="status-pill ${statusClass(task.status)}">${escapeHTML(task.status)}</span>
        </div>
        <div class="list-meta">
          <span>${escapeHTML(task.template_code || "--")}</span>
          <span>风险 ${escapeHTML(task.risk_level || "--")}</span>
          <span>${formatTime(task.created_at)}</span>
        </div>
      </article>
    `,
    "暂无 Task 记录。"
  );

  renderList(
    elements.templateList,
    templates,
    (template) => `
      <article class="list-card">
        <div class="list-row">
          <div>
            <h4>${escapeHTML(template.name)}</h4>
            <p>${escapeHTML(template.description || template.code)}</p>
          </div>
          <span class="status-pill">${escapeHTML(template.tool_type)}</span>
        </div>
        <div class="list-meta">
          <span>${escapeHTML(template.code)}</span>
          <span>风险 ${escapeHTML(template.risk_level)}</span>
        </div>
      </article>
    `,
    "暂无模板。"
  );
}

function renderSummaryMetrics(summary) {
  if (!elements.metricsGrid) {
    return;
  }
  const metrics = [
    ["在线 Workers", summary.metrics.workers_online, `${summary.workers.length} 个 Worker 已注册`],
    ["Jobs 总数", summary.metrics.jobs_total, `${summary.metrics.jobs_success} 成功 / ${summary.metrics.jobs_failed} 失败`],
    ["待审批", summary.metrics.jobs_waiting_approval, "高风险任务等待人工确认"],
    ["模板总数", summary.templates_total, "可用于 Task 解析"],
  ];

  elements.metricsGrid.innerHTML = metrics.map(([label, value, note]) => `
    <article class="metric-card">
      <p class="section-kicker">${escapeHTML(String(label))}</p>
      <strong>${escapeHTML(String(value))}</strong>
      <p>${escapeHTML(String(note))}</p>
    </article>
  `).join("");
}

function renderApprovals(items) {
  renderList(
    elements.approvalList,
    items,
    (job) => `
      <article class="list-card">
        <div class="list-row">
          <div>
            <h4>${escapeHTML(job.name)}</h4>
            <p>${escapeHTML(job.command || "无命令详情")}</p>
          </div>
          <span class="status-pill ${statusClass(job.status)}">${escapeHTML(job.status)}</span>
        </div>
        <div class="list-meta">
          <span>风险 ${escapeHTML(job.risk_level || "--")}</span>
          <span>${escapeHTML(job.worker_type)}</span>
          <span>${formatTime(job.created_at)}</span>
        </div>
        <div class="command-actions">
          <button class="primary-button" type="button" data-approval-id="${escapeHTML(job.id)}" data-decision="approve">批准</button>
          <button class="ghost-button" type="button" data-approval-id="${escapeHTML(job.id)}" data-decision="reject">拒绝</button>
        </div>
      </article>
    `,
    "当前没有待审批任务。"
  );
}

function renderAuditLogs(items) {
  renderList(
    elements.auditList,
    items,
    (log) => `
      <article class="list-card">
        <div class="list-row">
          <div>
            <h4>${escapeHTML(log.action)}</h4>
            <p>${escapeHTML(`${log.actor_type}:${log.actor_id} -> ${log.resource_type}:${log.resource_id}`)}</p>
          </div>
          <span class="status-pill">${escapeHTML(log.resource_type)}</span>
        </div>
        <div class="list-meta">
          <span>${formatTime(log.created_at)}</span>
        </div>
      </article>
    `,
    "暂无审计记录。"
  );
}

function renderLoggedOutPlaceholders() {
  if (elements.loginMessage && page === "login") {
    elements.loginMessage.textContent = "登录后即可进入用户、Workers、Jobs、Tasks 页面进行操作。";
  }
  renderList(elements.userList, [], () => "", "登录后显示用户列表。");
  renderList(elements.workerList, [], () => "", "登录后显示 Worker 列表。");
  renderList(elements.jobList, [], () => "", "登录后显示 Job 列表。");
  renderList(elements.taskList, [], () => "", "登录后显示 Task 记录。");
  renderList(elements.templateList, [], () => "", "登录后显示模板。");
  renderList(elements.approvalList, [], () => "", "登录后显示待审批任务。");
  renderList(elements.auditList, [], () => "", "登录后显示审计记录。");
  if (elements.metricsGrid) {
    elements.metricsGrid.innerHTML = "";
  }
  if (elements.taskOutput) {
    elements.taskOutput.textContent = "等待输入任务。";
  }
}

function renderList(container, items, renderer, emptyText) {
  if (!container) {
    return;
  }
  if (!items || items.length === 0) {
    container.innerHTML = `<div class="empty-state">${escapeHTML(emptyText)}</div>`;
    return;
  }
  container.innerHTML = items.map(renderer).join("");
}

function updateSessionState(serverTime) {
  if (!elements.sessionState) {
    return;
  }
  if (state.user?.username) {
    elements.sessionState.textContent = `${state.user.username} / ${state.user.role}`;
    if (elements.loginMessage && serverTime) {
      elements.loginMessage.textContent = `最近同步：${formatTime(serverTime)}`;
    }
    return;
  }
  elements.sessionState.textContent = "未登录";
}

function ensureAuthed() {
  if (state.token) {
    return true;
  }
  showToast("请先登录");
  if (page !== "login") {
    window.location.href = "/login";
  }
  return false;
}

async function authedRequest(url, options = {}) {
  return request(url, {
    ...options,
    headers: {
      ...(options.headers || {}),
      Authorization: `Bearer ${state.token}`,
    },
  });
}

async function request(url, options = {}) {
  const response = await window.fetch(url, {
    method: options.method || "GET",
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
    body: options.body,
  });

  const contentType = response.headers.get("content-type") || "";
  const payload = contentType.includes("application/json")
    ? await response.json()
    : await response.text();

  if (!response.ok) {
    const message = typeof payload === "string" ? payload : payload.error || "请求失败";
    const error = new Error(message);
    error.code = response.status;
    throw error;
  }

  return payload;
}

function valueOf(id) {
  return String(byId(id)?.value || "").trim();
}

function parseOptionalJSON(raw) {
  if (!raw) {
    return undefined;
  }
  try {
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed;
    }
    throw new Error("参数 JSON 必须是对象");
  } catch (_error) {
    throw new Error("参数 JSON 格式不正确");
  }
}

function formatTime(value) {
  if (!value) {
    return "--";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return String(value);
  }
  return date.toLocaleString("zh-CN", {
    hour12: false,
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function statusClass(status) {
  return `is-${String(status || "").toLowerCase()}`;
}

function showToast(message) {
  if (!elements.toast) {
    return;
  }
  window.clearTimeout(state.toastTimer);
  elements.toast.hidden = false;
  elements.toast.textContent = message;
  state.toastTimer = window.setTimeout(() => {
    elements.toast.hidden = true;
  }, 2400);
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function byId(id) {
  return document.getElementById(id);
}
