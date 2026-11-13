(() => {
  const state = document.getElementById("connection-state");
  const messagePanel = document.getElementById("message-panel");
  const messageTitle = document.getElementById("message-title");
  const messageDetail = document.getElementById("message-detail");
  const picker = document.getElementById("pane-picker");
  const paneList = document.getElementById("pane-list");
  const terminalPanel = document.getElementById("terminal-panel");
  const terminalElement = document.getElementById("terminal");
  const targetElement = document.getElementById("terminal-target");
  const modeElement = document.getElementById("terminal-mode");
  const backButton = document.getElementById("back-button");

  const parts = window.location.pathname.split("/").filter(Boolean);
  const session = parts.length === 2 && parts[0] === "sessions" ? parts[1] : "";
  const requestedPane = new URLSearchParams(window.location.search).get("pane") || "";
  let terminal = null;
  let fitAddon = null;
  let socket = null;
  let resizeObserver = null;
  let activeMode = "";
  let activePane = null;

  function setMessage(title, detail, isError = false) {
    messagePanel.hidden = false;
    messageTitle.textContent = title;
    messageDetail.textContent = detail;
    messagePanel.dataset.error = String(isError);
  }

  function setStatus(text) {
    state.textContent = text;
  }

  function closeTerminal() {
    if (socket && socket.readyState === WebSocket.OPEN && activeMode === "control") {
      socket.send(JSON.stringify({ type: "release" }));
    }
    if (socket) socket.close();
    if (resizeObserver) resizeObserver.disconnect();
    if (terminal) terminal.dispose();
    terminal = null;
    fitAddon = null;
    socket = null;
    activeMode = "";
    activePane = null;
    terminalPanel.hidden = true;
  }

  function showPicker() {
    closeTerminal();
    picker.hidden = false;
    messagePanel.hidden = true;
  }

  function createAction(label, className, action) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = className;
    button.textContent = label;
    button.addEventListener("click", action);
    return button;
  }

  function renderPanes(panes) {
    paneList.replaceChildren();
    for (const pane of panes) {
      const card = document.createElement("article");
      card.className = "pane-card";

      const name = document.createElement("p");
      name.className = "pane-name";
      name.textContent = pane.label || pane.id;
      card.append(name);

      const id = document.createElement("p");
      id.className = "pane-id";
      id.textContent = pane.id;
      card.append(id);

      if (pane.cwd) {
        const cwd = document.createElement("p");
        cwd.className = "pane-cwd";
        cwd.textContent = pane.cwd;
        card.append(cwd);
      }

      const actions = document.createElement("div");
      actions.className = "pane-actions";
      actions.append(createAction("入力して接続", "primary-button", () => connect(pane, "control")));
      actions.append(createAction("読み取り専用", "secondary-button", () => connect(pane, "observe")));
      card.append(actions);
      paneList.append(card);
    }
    picker.hidden = false;
    messagePanel.hidden = true;
  }

  function connect(pane, mode) {
    closeTerminal();
    activePane = pane;
    activeMode = mode;
    picker.hidden = true;
    messagePanel.hidden = true;
    terminalPanel.hidden = false;
    targetElement.textContent = `${session} · ${pane.label || pane.id} (${pane.id})`;
    modeElement.textContent = mode === "control" ? "入力・表示・サイズ変更が有効です" : "読み取り専用です";
    terminal = new window.Terminal({
      cursorBlink: mode === "control",
      convertEol: true,
      scrollback: 10000,
      fontFamily: "ui-monospace, SFMono-Regular, Consolas, monospace",
      fontSize: 14,
      theme: { background: "#0d1117", foreground: "#e8edf2", cursor: "#66d9b2" },
      disableStdin: mode !== "control",
    });
    fitAddon = new window.FitAddon.FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(terminalElement);
    fitAddon.fit();
    setStatus(mode === "control" ? "制御接続中…" : "読み取り接続中…");

    const scheme = window.location.protocol === "https:" ? "wss:" : "ws:";
    const query = new URLSearchParams({ session, pane: pane.id, mode });
    socket = new WebSocket(`${scheme}//${window.location.host}/ws?${query.toString()}`);
    socket.binaryType = "arraybuffer";
    socket.addEventListener("open", () => {
      setStatus(mode === "control" ? "入力可能" : "読み取り専用");
      if (mode === "control") sendResize();
    });
    socket.addEventListener("message", (event) => {
      if (typeof event.data !== "string") {
        terminal.write(new Uint8Array(event.data));
        return;
      }
      let message;
      try { message = JSON.parse(event.data); } catch { return; }
      if (message.type === "error") {
        setStatus("接続エラー");
        setMessage("端末 stream を開始できません", message.message || "Herdr session と pane を確認してください。", true);
      } else if (message.type === "closed") {
        setStatus("Herdr が接続を閉じました");
      }
    });
    socket.addEventListener("close", (event) => {
      if (event.code === 1006 && mode === "control") {
        const detail = "制御接続できません。pane が別の場所で制御中なら、読み取り専用で開いてください。";
        setStatus(detail);
        setMessage("制御接続できません", detail, true);
      } else if (event.code === 1006) {
        const detail = "Herdr に接続できません。セッションと pane を確認してください。";
        setStatus(detail);
        setMessage("Herdr に接続できません", detail, true);
      }
    });
    socket.addEventListener("error", () => setStatus("WebSocket 接続に失敗しました"));

    terminal.onData((text) => {
      if (mode === "control" && socket && socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "input", text }));
      }
    });
    terminal.onResize(({ cols, rows }) => {
      if (mode === "control" && socket && socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "resize", cols, rows }));
      }
    });
    resizeObserver = new ResizeObserver(() => {
      if (!fitAddon) return;
      fitAddon.fit();
      if (mode === "control") sendResize();
    });
    resizeObserver.observe(terminalElement);
  }

  function sendResize() {
    if (!terminal || !socket || socket.readyState !== WebSocket.OPEN || activeMode !== "control") return;
    socket.send(JSON.stringify({ type: "resize", cols: terminal.cols, rows: terminal.rows }));
  }

  async function loadPanes() {
    if (!session) {
      setStatus("セッション未指定");
      setMessage("セッションを指定してください", "myherdr url <session> で、このローカルサーバーの URL を取得してください。", true);
      return;
    }
    setStatus("pane を検索中…");
    setMessage("セッションを確認しています", "Herdr の既存 pane を検索しています。");
    try {
      const response = await fetch(`/api/sessions/${encodeURIComponent(session)}/panes`, { cache: "no-store" });
      if (!response.ok) {
        const detail = await response.text();
        throw new Error(detail || `HTTP ${response.status}`);
      }
      const result = await response.json();
      const panes = Array.isArray(result.panes) ? result.panes : [];
      if (requestedPane) {
        const requested = panes.find((pane) => pane.id === requestedPane);
        if (!requested) {
          setStatus("pane がありません");
          setMessage("指定された pane を利用できません", "Herdr セッションに pane が存在しないか、pane ID が変わっています。myherdr url <session> で一覧を開いてください。", true);
          return;
        }
        connect(requested, "control");
        return;
      }
      if (result.recommended) {
        const recommended = panes.find((pane) => pane.id === result.recommended);
        if (recommended) {
          connect(recommended, "control");
          return;
        }
      }
      if (panes.length === 0) {
        setStatus("接続できる pane がありません");
        setMessage("Herdr pane がありません", "Web アプリは pane を作成しません。Herdr CLI で既存 pane を用意してから、このページを再読み込みしてください。", true);
        return;
      }
      setStatus(`${panes.length} 個の pane`);
      renderPanes(panes);
    } catch (error) {
      setStatus("セッションに接続できません");
      setMessage("Herdr に接続できません", String(error.message || error), true);
    }
  }

  backButton.addEventListener("click", () => {
    showPicker();
    loadPanes();
  });
  window.addEventListener("resize", () => {
    if (fitAddon) {
      fitAddon.fit();
      sendResize();
    }
  });
  window.addEventListener("pagehide", closeTerminal);
  loadPanes();
})();
