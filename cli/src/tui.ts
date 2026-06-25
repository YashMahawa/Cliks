import blessed from "blessed";
import { runAutostart } from "./autostart.js";
import { saveConfig, toWsUrl, type CliksConfig } from "./config.js";

type ListeningState = CliksConfig["listening"];

type StartDashboardState = {
  teamName: string;
  activeCount: number;
  hearingSelf: boolean | undefined;
  listening: Pick<ListeningState, "volume" | "muted" | "spatial" | "fatigueProtection" | "density">;
  captureMode: string;
  connectionStatus: string;
  localCapturedEvents: number;
  localSentEvents: number;
  permissionHint?: string;
  notice?: string;
};

type StartDashboardActions = {
  adjustVolume(delta: number): void;
  adjustDensity(delta: number): void;
  toggle(key: "muted" | "spatial" | "fatigueProtection"): void;
  quit(): void;
};

const accent = "cyan";
const ok = "green";
const warn = "yellow";
const dim = "gray";

export class StartDashboard {
  private readonly screen: blessed.Widgets.Screen;
  private readonly header: blessed.Widgets.BoxElement;
  private readonly status: blessed.Widgets.BoxElement;
  private readonly meters: blessed.Widgets.BoxElement;
  private readonly controls: blessed.Widgets.BoxElement;
  private readonly log: blessed.Widgets.Log;
  private readonly actions: StartDashboardActions;
  private state: StartDashboardState | undefined;
  private closed = false;

  constructor(actions: StartDashboardActions) {
    this.actions = actions;
    this.screen = blessed.screen({
      smartCSR: true,
      fullUnicode: true,
      mouse: true,
      title: "Cliks"
    });

    this.header = blessed.box({
      parent: this.screen,
      top: 0,
      left: 0,
      width: "100%",
      height: 5,
      tags: true,
      style: { fg: "white", bg: "black" }
    });

    this.status = blessed.box({
      parent: this.screen,
      top: 5,
      left: 0,
      width: "52%",
      height: "50%-3",
      border: "line",
      label: " Room ",
      tags: true,
      padding: { left: 1, right: 1 },
      style: borderStyle()
    });

    this.meters = blessed.box({
      parent: this.screen,
      top: 5,
      left: "52%",
      width: "48%",
      height: "50%-3",
      border: "line",
      label: " Sound ",
      tags: true,
      padding: { left: 1, right: 1 },
      style: borderStyle()
    });

    this.controls = blessed.box({
      parent: this.screen,
      top: "50%+2",
      left: 0,
      width: "100%",
      height: 8,
      border: "line",
      label: " Controls ",
      tags: true,
      padding: { left: 1, right: 1 },
      style: borderStyle()
    });

    this.log = blessed.log({
      parent: this.screen,
      top: "50%+10",
      left: 0,
      width: "100%",
      bottom: 0,
      border: "line",
      label: " Hints ",
      tags: true,
      padding: { left: 1, right: 1 },
      scrollable: true,
      alwaysScroll: true,
      mouse: true,
      scrollbar: { ch: " ", track: { bg: "black" }, style: { bg: accent } },
      style: borderStyle()
    });

    this.bindKeys();
    this.buildControlButtons();
  }

  update(state: StartDashboardState) {
    if (this.closed) return;
    const previousNotice = this.state?.notice;
    const previousPermissionHint = this.state?.permissionHint;
    this.state = state;
    this.header.setContent(renderHeader(state));
    this.status.setContent(renderRoom(state));
    this.meters.setContent(renderSound(state));
    if (state.notice && state.notice !== previousNotice) {
      this.log.log(state.notice);
    }
    if (state.permissionHint && state.permissionHint !== previousPermissionHint) {
      this.log.log(state.permissionHint);
    }
    this.screen.render();
  }

  addLog(message: string) {
    if (this.closed) return;
    this.log.log(message);
    this.screen.render();
  }

  close() {
    if (this.closed) return;
    this.closed = true;
    this.screen.destroy();
  }

  private bindKeys() {
    this.screen.key(["C-c", "q"], () => this.actions.quit());
    this.screen.key(["up", "+"], () => this.actions.adjustVolume(0.05));
    this.screen.key(["down", "-"], () => this.actions.adjustVolume(-0.05));
    this.screen.key(["right", "]"], () => this.actions.adjustDensity(0.1));
    this.screen.key(["left", "["], () => this.actions.adjustDensity(-0.1));
    this.screen.key(["m"], () => this.actions.toggle("muted"));
    this.screen.key(["s"], () => this.actions.toggle("spatial"));
    this.screen.key(["f"], () => this.actions.toggle("fatigueProtection"));
  }

  private buildControlButtons() {
    const buttons: Array<{ text: string; left: number; width: number; onPress(): void }> = [
      { text: "Vol -", left: 0, width: 8, onPress: () => this.actions.adjustVolume(-0.05) },
      { text: "Vol +", left: 9, width: 8, onPress: () => this.actions.adjustVolume(0.05) },
      { text: "Den -", left: 18, width: 8, onPress: () => this.actions.adjustDensity(-0.1) },
      { text: "Den +", left: 27, width: 8, onPress: () => this.actions.adjustDensity(0.1) },
      { text: "Mute", left: 36, width: 8, onPress: () => this.actions.toggle("muted") },
      { text: "Space", left: 45, width: 9, onPress: () => this.actions.toggle("spatial") },
      { text: "Fade", left: 55, width: 8, onPress: () => this.actions.toggle("fatigueProtection") },
      { text: "Quit", left: 64, width: 8, onPress: () => this.actions.quit() }
    ];

    for (const item of buttons) {
      const button = blessed.button({
        parent: this.controls,
        top: 1,
        left: item.left,
        width: item.width,
        height: 3,
        mouse: true,
        keys: true,
        shrink: false,
        align: "center",
        valign: "middle",
        content: item.text,
        style: {
          fg: "white",
          bg: "black",
          border: { fg: accent },
          hover: { bg: "blue" },
          focus: { bg: "blue" }
        },
        border: "line"
      });
      button.on("press", item.onPress);
    }
    this.controls.setContent("\n\n\n\nUse arrow keys, shortcut letters, or click a button. Mouse wheel adjusts volume.");
    this.screen.on("wheelup", () => this.actions.adjustVolume(0.05));
    this.screen.on("wheeldown", () => this.actions.adjustVolume(-0.05));
  }
}

export async function runSettingsTui(config: CliksConfig) {
  if (!process.stdout.isTTY || !process.stdin.isTTY) {
    printSettingsPlain(config);
    return;
  }

  const screen = blessed.screen({
    smartCSR: true,
    fullUnicode: true,
    mouse: true,
    title: "Cliks Settings"
  });

  let dirty = false;
  let selected = 0;
  let message = "Arrows or mouse to select. Left/right change values. Enter toggles. s saves.";

  const items = buildSettingsItems(config);

  const title = blessed.box({
    parent: screen,
    top: 0,
    left: 0,
    height: 3,
    width: "100%",
    tags: true
  });

  const list = blessed.list({
    parent: screen,
    top: 3,
    left: 0,
    width: "58%",
    bottom: 3,
    border: "line",
    label: " Settings ",
    mouse: true,
    keys: true,
    vi: true,
    tags: true,
    padding: { left: 1, right: 1 },
    style: {
      ...borderStyle(),
      selected: { bg: "blue", fg: "white", bold: true },
      item: { fg: "white" }
    }
  });

  const detail = blessed.box({
    parent: screen,
    top: 3,
    left: "58%",
    width: "42%",
    bottom: 3,
    border: "line",
    label: " Detail ",
    tags: true,
    padding: { left: 1, right: 1 },
    scrollable: true,
    mouse: true,
    style: borderStyle()
  });

  const footer = blessed.box({
    parent: screen,
    bottom: 0,
    left: 0,
    height: 3,
    width: "100%",
    tags: true,
    border: "line",
    padding: { left: 1, right: 1 },
    style: borderStyle()
  });

  function render() {
    const rows = items.map((item) => `${item.label.padEnd(18)} ${item.value()}`);
    title.setContent(
      `{bold}Cliks Settings{/bold}\n` +
        `${config.currentTeamCode ? `Team: {${accent}-fg}${config.currentTeamCode}{/${accent}-fg}` : "No team selected"}    ` +
        `${config.nickname ? `Nickname: ${config.nickname}` : "Nickname: not set"}`
    );
    list.setItems(rows);
    list.select(selected);
    detail.setContent(items[selected]?.detail() ?? "");
    footer.setContent(
      `${dirty ? `{${warn}-fg}Unsaved changes{/${warn}-fg}` : `{${ok}-fg}Saved{/${ok}-fg}`}  ${message}\n` +
        "Keys: ↑/↓ select  ←/→ adjust  Enter toggle/edit  s save  a autostart  d doctor  q quit"
    );
    screen.render();
  }

  async function save() {
    await saveConfig(config);
    dirty = false;
    message = "Saved settings.";
    render();
  }

  function change(delta: number) {
    const item = items[selected];
    if (!item) return;
    item.change(delta);
    dirty = true;
    render();
  }

  async function edit() {
    const item = items[selected];
    if (!item) return;
    if (item.toggle) {
      item.toggle();
      dirty = true;
      render();
      return;
    }
    if (!item.prompt) return;
    const current = item.raw();
    const value = await prompt(screen, item.label, current);
    if (value !== undefined) {
      item.prompt(value);
      dirty = true;
      render();
    }
  }

  list.on("select", (_element, index) => {
    selected = Number(index);
    void edit();
  });
  list.on("select item", (_element, index) => {
    selected = Number(index);
    render();
  });

  screen.key(["up", "k"], () => {
    selected = Math.max(0, selected - 1);
    render();
  });
  screen.key(["down", "j"], () => {
    selected = Math.min(items.length - 1, selected + 1);
    render();
  });
  screen.key(["left", "h"], () => change(-1));
  screen.key(["right", "l"], () => change(1));
  screen.key(["enter", "space"], () => {
    void edit();
  });
  screen.key(["s"], () => {
    void save();
  });
  screen.key(["a"], () => {
    void (async () => {
      await save();
      await runAutostart("enable", config, config.currentTeamCode);
      message = "Autostart enable requested for the current team.";
      render();
    })();
  });
  screen.key(["d"], () => {
    message = "Run typ doctor in a normal terminal for the full permission check.";
    render();
  });
  screen.key(["q", "escape", "C-c"], () => {
    if (dirty) {
      void save().finally(() => screen.destroy());
    } else {
      screen.destroy();
    }
  });

  render();
  await new Promise<void>((resolve) => screen.on("destroy", resolve));
}

type SettingsItem = {
  label: string;
  value(): string;
  detail(): string;
  raw(): string;
  change(delta: number): void;
  toggle?(): void;
  prompt?(value: string): void;
};

function buildSettingsItems(config: CliksConfig): SettingsItem[] {
  return [
    numberItem("Volume", () => config.listening.volume, (value) => (config.listening.volume = value), 0, 1, 0.05, "Overall playback volume."),
    numberItem("Density", () => config.listening.density, (value) => (config.listening.density = value), 0.15, 1, 0.05, "Local thinning for dense bursts. It changes what you hear, not what you send."),
    boolItem("Muted", () => config.listening.muted, (value) => (config.listening.muted = value), "Silence local playback without disconnecting from the room."),
    boolItem("Spatial audio", () => config.listening.spatial, (value) => (config.listening.spatial = value), "Use pan and distance when the detected audio player supports it."),
    boolItem("Fatigue fade", () => config.listening.fatigueProtection, (value) => (config.listening.fatigueProtection = value), "Softens long dense typing runs so the room stays comfortable."),
    boolItem("Hear keyboard", () => config.listening.keyboard, (value) => (config.listening.keyboard = value), "Play teammate keyboard activity."),
    boolItem("Hear mouse", () => config.listening.mouse, (value) => (config.listening.mouse = value), "Play teammate left/right mouse clicks."),
    boolItem("Self monitor", () => config.listening.self, (value) => (config.listening.self = value), "Hear your own pulses for testing."),
    boolItem("Share keyboard", () => config.sharing.keyboard, (value) => (config.sharing.keyboard = value), "Send keyboard activity pulses. Key values are never sent."),
    boolItem("Share mouse", () => config.sharing.mouse, (value) => (config.sharing.mouse = value), "Send left/right mouse click pulses."),
    numberItem("Batch window", () => config.batchWindowMs, (value) => (config.batchWindowMs = Math.round(value)), 100, 2000, 50, "Local batching window in milliseconds."),
    textItem("Nickname", () => config.nickname ?? "", (value) => (config.nickname = value.trim() || undefined), "Shown to teammates when presence UIs support names."),
    teamItem(config),
    textItem("API URL", () => config.apiUrl, (value) => {
      config.apiUrl = value.trim().replace(/\/$/, "");
      config.wsUrl = toWsUrl(config.apiUrl);
    }, "Backend HTTP URL. Updating this also updates the WebSocket URL."),
    textItem("WebSocket URL", () => config.wsUrl, (value) => (config.wsUrl = value.trim()), "Advanced override for the relay WebSocket.")
  ];
}

function numberItem(
  label: string,
  get: () => number,
  set: (value: number) => void,
  min: number,
  max: number,
  step: number,
  help: string
): SettingsItem {
  return {
    label,
    value: () => (max <= 1 ? progress(get(), min, max) : `${Math.round(get())}ms`),
    detail: () => `${help}\n\nCurrent: ${max <= 1 ? `${Math.round(get() * 100)}%` : `${Math.round(get())}ms`}`,
    raw: () => String(get()),
    change: (delta) => set(clamp(get() + delta * step, min, max)),
    prompt: (value) => {
      const numeric = Number(value);
      if (Number.isFinite(numeric)) set(clamp(numeric, min, max));
    }
  };
}

function boolItem(
  label: string,
  get: () => boolean,
  set: (value: boolean) => void,
  help: string
): SettingsItem {
  return {
    label,
    value: () => (get() ? `{${ok}-fg}on{/${ok}-fg}` : `{${dim}-fg}off{/${dim}-fg}`),
    detail: () => `${help}\n\nClick or press Enter to toggle.`,
    raw: () => (get() ? "on" : "off"),
    change: () => set(!get()),
    toggle: () => set(!get())
  };
}

function textItem(label: string, get: () => string, set: (value: string) => void, help: string): SettingsItem {
  return {
    label,
    value: () => get() || `{${dim}-fg}not set{/${dim}-fg}`,
    detail: () => `${help}\n\nPress Enter to edit.`,
    raw: get,
    change: () => undefined,
    prompt: set
  };
}

function teamItem(config: CliksConfig): SettingsItem {
  return {
    label: "Current team",
    value: () => config.currentTeamCode ?? `{${dim}-fg}not set{/${dim}-fg}`,
    detail: () => {
      const teams = config.teams.map((team) => `${team.code}${team.name ? `  ${team.name}` : ""}`).join("\n");
      return `Selected room for typ start and autostart.\n\nSaved teams:\n${teams || "No saved teams yet."}`;
    },
    raw: () => config.currentTeamCode ?? "",
    change: (delta) => {
      if (config.teams.length === 0) return;
      const currentIndex = Math.max(0, config.teams.findIndex((team) => team.code === config.currentTeamCode));
      const nextIndex = (currentIndex + delta + config.teams.length) % config.teams.length;
      config.currentTeamCode = config.teams[nextIndex]?.code;
    },
    prompt: (value) => {
      const code = value.trim().toUpperCase();
      if (!code) {
        config.currentTeamCode = undefined;
        return;
      }
      config.currentTeamCode = code;
      if (!config.teams.some((team) => team.code === code)) {
        config.teams.unshift({ code, lastJoinedAt: new Date().toISOString() });
      }
    }
  };
}

function renderHeader(state: StartDashboardState) {
  const connColor = state.connectionStatus === "connected" ? ok : state.connectionStatus.includes("error") ? "red" : warn;
  return (
    `{bold}Cliks{/bold}  {${accent}-fg}${state.teamName}{/${accent}-fg}\n` +
    `{${connColor}-fg}${state.connectionStatus}{/${connColor}-fg}  ` +
    `${state.activeCount} active  ` +
    `capture ${state.captureMode}\n` +
    `{${dim}-fg}Only activity type and coarse timing are sent. No keys, text, windows, coordinates, screen, or audio.{/${dim}-fg}`
  );
}

function renderRoom(state: StartDashboardState) {
  const lines = [
    `Team: {${accent}-fg}${state.teamName}{/${accent}-fg}`,
    `Active now: {bold}${state.activeCount}{/bold}`,
    `Connection: ${state.connectionStatus}`,
    `Capture: ${state.captureMode}`,
    `Self monitor: ${state.hearingSelf ? "on" : "off"}`,
    "",
    `Captured locally: {bold}${state.localCapturedEvents}{/bold}`,
    `Sent to room:     {bold}${state.localSentEvents}{/bold}`
  ];
  if (state.captureMode === "terminal") {
    lines.push("", "Terminal mode affects this terminal only.");
    lines.push("Run typ fix-terminal if input feels stuck.");
  }
  if (state.captureMode !== "off" && state.localCapturedEvents === 0) {
    lines.push("", `{${warn}-fg}No local activity captured yet. Try typ capture-test if this stays at 0.{/${warn}-fg}`);
  }
  return lines.join("\n");
}

function renderSound(state: StartDashboardState) {
  const listening = state.listening;
  return [
    `Volume   ${listening.muted ? `{${warn}-fg}muted{/${warn}-fg}` : progress(listening.volume, 0, 1)}`,
    `Density  ${progress(listening.density ?? 1, 0.15, 1)}`,
    "",
    `Spatial audio  ${listening.spatial === false ? `{${dim}-fg}off{/${dim}-fg}` : `{${ok}-fg}on{/${ok}-fg}`}`,
    `Fatigue fade   ${listening.fatigueProtection === false ? `{${dim}-fg}off{/${dim}-fg}` : `{${ok}-fg}on{/${ok}-fg}`}`,
    "",
    "Keyboard: Up/Down volume, Left/Right density",
    "Mouse: click buttons below"
  ].join("\n");
}

function progress(value: number, min: number, max: number) {
  const normalized = clamp((value - min) / (max - min), 0, 1);
  const width = 18;
  const filled = Math.round(normalized * width);
  return `{${accent}-fg}${"█".repeat(filled)}{/${accent}-fg}{${dim}-fg}${"░".repeat(width - filled)}{/${dim}-fg} ${Math.round(value * 100)}%`;
}

function borderStyle() {
  return {
    fg: "white",
    bg: "black",
    border: { fg: accent }
  };
}

async function prompt(screen: blessed.Widgets.Screen, label: string, current: string) {
  return new Promise<string | undefined>((resolve) => {
    const question = blessed.prompt({
      parent: screen,
      border: "line",
      height: 7,
      width: "70%",
      top: "center",
      left: "center",
      label: ` ${label} `,
      tags: true,
      keys: true,
      mouse: true,
      inputOnFocus: true,
      padding: { left: 1, right: 1 },
      style: borderStyle()
    });
    question.input(`${label}:`, current, (error, value) => {
      question.destroy();
      screen.render();
      if (error) resolve(undefined);
      else resolve(value);
    });
  });
}

function printSettingsPlain(config: CliksConfig) {
  console.log("Cliks settings");
  console.log(JSON.stringify(config, null, 2));
  console.log("");
  console.log("Run typ settings in an interactive terminal for mouse and keyboard controls.");
}

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value));
}
