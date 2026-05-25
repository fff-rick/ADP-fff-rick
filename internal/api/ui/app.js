const state = {
  token: window.localStorage.getItem("adp.token") || "",
  user: null,
  lastPlan: null,
  toastTimer: null,
  refreshTimer: null,
};

const elements = {
  clock: document.getElementById("clock"),
  sessionState: document.getElementById("session-state"),
  loginToggle: document.getElementById("login-toggle"),
  loginPanel: document.getElementById("login-panel"),
  loginForm: document.getElementById("login-form"),
  loginMessage: document.getElementById("login-message"),
  logoutButton: document.getElementById("logout-button"),
  commandForm: document.getElementById("command-form"),
  commandInput: document.getElementById("command-input"),
  executePlan: document.getElementById("execute-plan"),
  metricsGrid: document.getElementById("metrics-grid"),
  railSteps: document.getElementById("rail-steps"),
  railOutput: document.getElementById("rail-output"),
  pendingApprovalCount: document.getElementById("pending-approval-count"),
  templatesTotal: document.getElementById("templates-total"),
  approvalList: document.getElementById("approval-list"),
  jobList: document.getElementById("job-list"),
  caseList: document.getElementById("case-list"),
  auditList: document.getElementById("audit-list"),
  toast: document.getElementById("toast"),
};

boot();

function boot() {
  bindEvents();
  startClock();
  setRail(defaultRailSteps(), "等待输入任务或登录后加载实时数据。");
  renderEmptyDashboard();
  updateSessionState();

  if (state.token) {
    refreshSummary();
    state.refreshTimer = window.setInterval(refreshSummary, 12000);
  }
}

function bindEvents() {
  elements.loginToggle.addEventListener("click", () => toggleLoginPanel());
  elements.logoutButton.addEventListener("click", handleLogout);
  elements.loginPanel.addEventListener("click", (event) => {
    if (event.target === elements.loginPanel) {
      toggleLoginPanel(false);
    }
  });

  elements.loginForm.addEventListener("submit", handleLogin);
  elements.commandForm.addEventListener("submit", handleCommandSubmit);
  elements.executePlan.addEventListener("click", executeLatestPlan);

  document.querySelectorAll(".scenario-chip").forEach((button) => {
    button.addEventListener("click", () => {
      elements.commandInput.value = button.dataset.prompt || "";
      elements.commandInput.focus();
    });
  });

  elements.approvalList.addEventListener("click", async (event) => {
    const button = event.target.closest("[data-approval-id]");
    if (!button) {
      return;
    }

    const jobID = button.dataset.approvalId;
    const approved = button.dataset.decision === "approve";
    await decideApproval(jobID, approved);
  });
}

function startClock() {
  const tick = () => {
    elements.clock.textContent = new Date().toLocaleTimeString("zh-CN", {
      hour12: false,
    });
  };

  tick();
  window.setInterval(tick, 1000);
}

function toggleLoginPanel(force) {
  const shouldOpen = typeof force === "boolean"
    ? force
    : !elements.loginPanel.classList.contains("is-open");

  elements.loginPanel.classList.toggle("is-open", shouldOpen);
  elements.loginPanel.setAttribute("aria-hidden", shouldOpen ? "false" : "true");
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
    elements.loginMessage.textContent = "登录成功，正在刷新控制台。";
    toggleLoginPanel(false);
    showToast("已登录，实时数据已解锁");
    refreshSummary();

    if (!state.refreshTimer) {
      state.refreshTimer = window.setInterval(refreshSummary, 12000);
    }
  } catch (error) {
    elements.loginMessage.textContent = error.message;
    showToast(error.message);
  }
}

function handleLogout() {
  state.token = "";
  state.user = null;
  state.lastPlan = null;
  window.localStorage.removeItem("adp.token");
  elements.executePlan.hidden = true;
  elements.loginMessage.textContent = "已退出登录。";

  if (state.refreshTimer) {
    window.clearInterval(state.refreshTimer);
    state.refreshTimer = null;
  }

  updateSessionState();
  renderEmptyDashboard();
  setRail(defaultRailSteps(), "当前未登录，页面仅展示结构与基础状态。");
  showToast("登录状态已清除");
}

async function refreshSummary() {
  if (!state.token) {
    return;
  }

  try {
    const summary = await authedRequest("/api/v1/dashboard/summary");
    state.user = summary.user;
    updateSessionState(summary.current_time);
    renderSummary(summary);
  } catch (error) {
    if (error.code === 401) {
      handleLogout();
    }
  }
}

async function handleCommandSubmit(event) {
  event.preventDefault();

  const input = elements.commandInput.value.trim();
  if (!input) {
    showToast("先输入任务目标或故障描述");
    return;
  }
  if (!ensureAuthed()) {
    return;
  }

  const action = event.submitter?.dataset.action || "parse";
  const outputPrefix = `> ${input}\n\n`;

  try {
    if (action === "parse") {
      const result = await authedRequest("/api/v1/tasks/parse", {
        method: "POST",
        body: JSON.stringify({ input }),
      });

      const steps = defaultRailSteps();
      steps[0].state = "done";
      steps[0].detail = "已识别任务目标。";
      steps[1].state = "done";
      steps[1].detail = `${result.intent} / ${result.target_type}`;
      steps[2].state = result.risk_level === "high" ? "warn" : "done";
      steps[2].detail = `风险等级：${result.risk_level}`;
      setRail(steps, outputPrefix + JSON.stringify(result, null, 2));
      showToast("任务解析完成");
      return;
    }

    if (action === "run") {
      const result = await authedRequest("/api/v1/tasks/run", {
        method: "POST",
        body: JSON.stringify({ input }),
      });

      const steps = defaultRailSteps();
      steps[0].state = "done";
      steps[0].detail = "输入已接收。";
      steps[1].state = "done";
      steps[1].detail = `${result.parsed_intent.intent} / 模板 ${result.template_code}`;
      steps[2].state = result.approval_required ? "warn" : "done";
      steps[2].detail = result.approval_required ? "任务进入待审批队列。" : "已通过策略校验。";
      steps[3].state = "done";
      steps[3].detail = `任务 ${result.job.id} 已创建。`;
      steps[4].state = result.approval_required ? "warn" : "active";
      steps[4].detail = result.approval_required ? "等待审批后执行。" : "等待 Worker 拉取执行。";
      setRail(steps, outputPrefix + JSON.stringify(result, null, 2));
      showToast(result.approval_required ? "任务已进入审批队列" : "任务已成功入队");
      refreshSummary();
      return;
    }

    if (action === "plan") {
      const result = await authedRequest("/api/v1/diagnosis/plan", {
        method: "POST",
        body: JSON.stringify({ description: input }),
      });

      state.lastPlan = result;
      elements.executePlan.hidden = false;

      const steps = defaultRailSteps();
      steps[0].state = "done";
      steps[0].detail = "故障描述已接收。";
      steps[1].state = "done";
      steps[1].detail = `已生成 ${result.steps.length} 个诊断步骤。`;
      steps[2].state = "active";
      steps[2].detail = "可继续执行，或进入审批流程。";
      setRail(steps, outputPrefix + JSON.stringify(result, null, 2));
      showToast("诊断计划已生成");
    }
  } catch (error) {
    setRail(markRailFailed(), outputPrefix + error.message);
    showToast(error.message);
  }
}

async function executeLatestPlan() {
  if (!state.lastPlan?.id) {
    showToast("当前没有可执行的诊断计划");
    return;
  }
  if (!ensureAuthed()) {
    return;
  }

  try {
    const result = await authedRequest(`/api/v1/diagnosis/plan/${state.lastPlan.id}/execute`, {
      method: "POST",
    });

    const steps = defaultRailSteps();
    steps[0].state = "done";
    steps[1].state = "done";
    steps[1].detail = state.lastPlan.title;
    steps[2].state = result.approval_required ? "warn" : "done";
    steps[2].detail = result.approval_required ? "部分步骤需要审批。" : "计划已通过策略校验。";
    steps[3].state = "done";
    steps[3].detail = `已创建 ${result.jobs.length} 个任务。`;
    steps[4].state = "active";
    steps[4].detail = "等待 Worker 执行并回传结果。";
    setRail(steps, JSON.stringify(result, null, 2));
    showToast("诊断计划已下发");
    refreshSummary();
  } catch (error) {
    setRail(markRailFailed(), error.message);
    showToast(error.message);
  }
}

async function decideApproval(jobID, approved) {
  if (!ensureAuthed()) {
    return;
  }

  try {
    const result = await authedRequest(`/api/v1/approvals/jobs/${jobID}`, {
      method: "POST",
      body: JSON.stringify({
        approved,
        comment: approved ? "Approved from dashboard" : "Rejected from dashboard",
      }),
    });

    const steps = defaultRailSteps();
    steps[2].state = approved ? "done" : "fail";
    steps[2].detail = approved ? "高风险动作已批准。" : "任务已拒绝。";
    steps[3].state = approved ? "active" : "fail";
    steps[3].detail = approved ? `任务 ${result.id} 已重新入队。` : `任务 ${result.id} 已取消。`;
    setRail(steps, JSON.stringify(result, null, 2));
    showToast(approved ? "审批已通过" : "审批已拒绝");
    refreshSummary();
  } catch (error) {
    showToast(error.message);
  }
}

function renderSummary(summary) {
  const successRate = `${Math.round(summary.metrics.job_success_rate * 100)}%`;
  const latency = `${summary.metrics.avg_schedule_latency_seconds.toFixed(2)}s`;
  const metrics = [
    {
      label: "Online Workers",
      value: summary.metrics.workers_online,
      note: `${summary.workers.length} 个 Worker 已注册`,
    },
    {
      label: "Tasks Today",
      value: summary.metrics.jobs_total,
      note: `模板数 ${summary.templates_total}`,
    },
    {
      label: "Success Rate",
      value: successRate,
      note: `${summary.metrics.jobs_success} 成功 / ${summary.metrics.jobs_failed} 失败`,
    },
    {
      label: "Approval Queue",
      value: summary.metrics.jobs_waiting_approval,
      note: "等待人工确认",
    },
    {
      label: "Case Memory",
      value: summary.metrics.incident_cases_total,
      note: "已沉淀历史经验",
    },
    {
      label: "Avg Latency",
      value: latency,
      note: "从入队到开始执行",
    },
  ];

  elements.metricsGrid.innerHTML = metrics.map((item) => `
    <article class="metric-card">
      <p class="section-kicker">${escapeHTML(item.label)}</p>
      <strong>${escapeHTML(String(item.value))}</strong>
      <p>${escapeHTML(item.note)}</p>
    </article>
  `).join("");

  elements.pendingApprovalCount.textContent = String(summary.pending_approvals.length);
  elements.templatesTotal.textContent = String(summary.templates_total);

  renderList(
    elements.approvalList,
    summary.pending_approvals,
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
          <span>${escapeHTML(job.risk_level || "unknown")}</span>
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

  renderList(
    elements.jobList,
    summary.recent_jobs,
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
          <span>${escapeHTML(job.worker_type)}</span>
          <span>${escapeHTML(job.template_code || job.source_type || "manual")}</span>
          <span>${formatTime(job.updated_at)}</span>
        </div>
      </article>
    `,
    "暂无任务记录。"
  );

  renderList(
    elements.caseList,
    summary.recent_cases,
    (incidentCase) => `
      <article class="list-card">
        <div class="list-row">
          <div>
            <h4>${escapeHTML(incidentCase.title)}</h4>
            <p>${escapeHTML(incidentCase.summary || "暂无摘要")}</p>
          </div>
          <span class="status-pill">${escapeHTML(incidentCase.trigger_type || "case")}</span>
        </div>
        <div class="list-meta">
          <span>${escapeHTML(incidentCase.fault_type || "unknown")}</span>
          <span>置信度 ${Math.round((incidentCase.confidence || 0) * 100)}%</span>
          <span>${formatTime(incidentCase.updated_at)}</span>
        </div>
      </article>
    `,
    "案例库还没有历史数据。"
  );

  renderList(
    elements.auditList,
    summary.recent_audit_logs,
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
    "审计记录为空。"
  );
}

function renderEmptyDashboard() {
  elements.metricsGrid.innerHTML = "";
  elements.pendingApprovalCount.textContent = "0";
  elements.templatesTotal.textContent = "0";
  renderList(elements.approvalList, [], () => "", "登录后显示待审批任务。");
  renderList(elements.jobList, [], () => "", "登录后显示最近任务。");
  renderList(elements.caseList, [], () => "", "登录后显示案例库内容。");
  renderList(elements.auditList, [], () => "", "登录后显示审计记录。");
}

function renderList(container, items, renderer, emptyText) {
  if (!items || items.length === 0) {
    container.innerHTML = `<div class="empty-state">${escapeHTML(emptyText)}</div>`;
    return;
  }

  container.innerHTML = items.map(renderer).join("");
}

function updateSessionState(serverTime) {
  if (state.user?.username) {
    elements.sessionState.textContent = `${state.user.username} / 在线`;
    if (serverTime) {
      elements.loginMessage.textContent = `最近同步：${formatTime(serverTime)}`;
    }
    return;
  }

  elements.sessionState.textContent = "未登录";
}

function setRail(steps, output) {
  elements.railSteps.innerHTML = steps.map((step) => `
    <article class="rail-step ${railClass(step.state)}">
      <div class="rail-marker"></div>
      <div>
        <h4>${escapeHTML(step.title)}</h4>
        <p>${escapeHTML(step.detail)}</p>
      </div>
    </article>
  `).join("");

  elements.railOutput.textContent = output;
}

function defaultRailSteps() {
  return [
    {
      title: "解析自然语言目标",
      detail: "识别意图与任务场景。",
      state: "idle",
    },
    {
      title: "生成结构化任务或计划",
      detail: "映射模板、参数与诊断步骤。",
      state: "idle",
    },
    {
      title: "策略校验与风险分级",
      detail: "检查白名单、模板合法性和审批要求。",
      state: "idle",
    },
    {
      title: "调度入队",
      detail: "将任务发送到对应类型的 Worker。",
      state: "idle",
    },
    {
      title: "Worker 执行",
      detail: "执行命令并回传结果。",
      state: "idle",
    },
    {
      title: "结果分析与沉淀",
      detail: "生成摘要、建议并写入案例库。",
      state: "idle",
    },
    {
      title: "审计回放",
      detail: "记录发起、审批与执行过程。",
      state: "idle",
    },
  ];
}

function markRailFailed() {
  const steps = defaultRailSteps();
  steps[2].state = "fail";
  steps[2].detail = "流程被错误或策略阻断。";
  return steps;
}

function railClass(stateName) {
  return {
    active: "is-active",
    done: "is-done",
    warn: "is-warn",
    fail: "is-fail",
  }[stateName] || "";
}

function statusClass(status) {
  return `is-${String(status || "").toLowerCase()}`;
}

function ensureAuthed() {
  if (state.token) {
    return true;
  }

  toggleLoginPanel(true);
  showToast("这个操作需要先登录");
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
    const message = typeof payload === "string"
      ? payload
      : payload.error || "请求失败";
    const error = new Error(message);
    error.code = response.status;
    throw error;
  }

  return payload;
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

function showToast(message) {
  window.clearTimeout(state.toastTimer);
  elements.toast.hidden = false;
  elements.toast.textContent = message;
  state.toastTimer = window.setTimeout(() => {
    elements.toast.hidden = true;
  }, 2200);
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
