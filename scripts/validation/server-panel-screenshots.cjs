/*
 * Headless Servers / Sandboxes UI validation.
 *
 * Starts a tracked release binary against isolated validation data, bootstraps
 * a random admin password through GLYPHDECK_ADMIN_PASSWORD, and captures only
 * authenticated ServersPanel states. Lifecycle success/error responses are
 * mocked in the browser because this validation must not require an SSH host.
 */
const { chromium } = require("playwright");
const crypto = require("crypto");
const fs = require("fs");
const http = require("http");
const path = require("path");
const { spawn } = require("child_process");

const REPO_ROOT = path.resolve(__dirname, "..", "..");
const VALIDATION_DIR = path.join(
  REPO_ROOT,
  ".glyphdeck",
  "validation",
  "remote-lifecycle",
);
const SCREENSHOT_DIR = path.join(VALIDATION_DIR, "screenshots");
const LOG_DIR = path.join(VALIDATION_DIR, "logs");
const PID_DIR = path.join(VALIDATION_DIR, "pids");
const DATA_DIR = path.join(VALIDATION_DIR, "data");
const BINARY = path.join(REPO_ROOT, "dist", "glyphdeck.exe");
const TARGET_ID = "remote-ui-validation";
const TARGET_URL = "http://127.0.0.1:4096";
const PASSWORD = crypto.randomBytes(24).toString("hex");
const captures = [];
const assertions = [];
let browser;
let backend;

function assertValidationPath(candidate) {
  const resolved = path.resolve(candidate);
  const prefix = `${path.resolve(VALIDATION_DIR)}${path.sep}`;
  if (resolved !== path.resolve(VALIDATION_DIR) && !resolved.startsWith(prefix)) {
    throw new Error(`Validation path escapes .glyphdeck/validation/remote-lifecycle: ${resolved}`);
  }
  return resolved;
}

function record(label) {
  assertions.push({ label, status: "PASS" });
  console.log(`[OK] ${label}`);
}

function fail(message) {
  assertions.push({ label: message, status: "FAIL" });
  throw new Error(message);
}

async function freePort() {
  return new Promise((resolve, reject) => {
    const listener = http.createServer();
    listener.once("error", reject);
    listener.listen(0, "127.0.0.1", () => {
      const { port } = listener.address();
      listener.close((error) => (error ? reject(error) : resolve(port)));
    });
  });
}

async function waitForHealth(baseUrl, timeoutMs = 20000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${baseUrl}/healthz`);
      if (response.ok) return;
    } catch {
      // Backend is still starting.
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  fail("tracked isolated backend did not reach /healthz");
}

async function ensureAbsent(page, testId, label) {
  if ((await page.getByTestId(testId).count()) !== 0) {
    fail(`${label}: unexpected ${testId}`);
  }
  record(label);
}

async function ensureServersPanel(page, label) {
  await ensureAbsent(page, "setup-screen", `${label}: setup screen absent`);
  await ensureAbsent(page, "login-screen", `${label}: login screen absent`);
  await page.getByTestId("servers-panel").waitFor({ state: "visible", timeout: 10000 });
  record(`${label}: authenticated Servers / Sandboxes panel visible`);
}

async function capture(page, name, state) {
  await ensureServersPanel(page, state);
  await page.evaluate(() => {
    window.scrollTo(0, 0);
    document.documentElement.scrollTop = 0;
    document.body.scrollTop = 0;
  });
  await page.getByTestId("left-panel-body").evaluate((element) => {
    element.scrollTop = 0;
  });
  await page.evaluate(
    () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
  );
  const target = assertValidationPath(path.join(SCREENSHOT_DIR, name));
  await page.screenshot({ path: target, fullPage: false, animations: "disabled" });
  captures.push({ name, state });
  console.log(`[SCREENSHOT] ${name}`);
}

function writeManifest(result, failure) {
  const lines = [
    "# Remote lifecycle UI validation manifest",
    "",
    "## Run",
    `- Result: ${result}`,
    "- Backend: isolated release binary on loopback with random bootstrap password.",
    "- Lifecycle success/error browser responses: mocked; no SSH host required.",
    "",
    "## Assertions",
    ...assertions.map((item) => `- ${item.status}: ${item.label}`),
    "",
    "## Screenshots",
    "",
    "| File | Verified state |",
    "|---|---|",
    ...captures.map((item) => `| ${item.name} | ${item.state} |`),
    "",
  ];
  if (failure) {
    lines.push("## Failure", "", `- ${failure.replace(/https?:\/\/\S+/g, "<url-removed>")}`, "");
  }
  fs.writeFileSync(
    assertValidationPath(path.join(SCREENSHOT_DIR, "manifest.md")),
    lines.join("\n"),
  );
}

async function setRoute(page, url, status, body) {
  await page.route(url, (route) =>
    route.fulfill({
      status,
      contentType: "application/json",
      body: JSON.stringify(body),
    }),
  );
}

async function run() {
  if (!fs.existsSync(BINARY)) {
    fail("dist/glyphdeck.exe is missing; run scripts/build.ps1 before UI validation");
  }

  for (const directory of [SCREENSHOT_DIR, LOG_DIR, PID_DIR]) {
    fs.mkdirSync(assertValidationPath(directory), { recursive: true });
  }
  fs.rmSync(assertValidationPath(DATA_DIR), { recursive: true, force: true });
  fs.mkdirSync(assertValidationPath(DATA_DIR), { recursive: true });
  for (const entry of fs.readdirSync(SCREENSHOT_DIR)) {
    if (entry.endsWith(".png") || entry === "manifest.md") {
      fs.rmSync(assertValidationPath(path.join(SCREENSHOT_DIR, entry)), { force: true });
    }
  }

  const port = await freePort();
  const baseUrl = `http://127.0.0.1:${port}`;
  const stdout = fs.openSync(assertValidationPath(path.join(LOG_DIR, "backend.log")), "w");
  const stderr = fs.openSync(assertValidationPath(path.join(LOG_DIR, "backend-error.log")), "w");
  backend = spawn(BINARY, [], {
    cwd: VALIDATION_DIR,
    detached: false,
    windowsHide: true,
    stdio: ["ignore", stdout, stderr],
    env: {
      ...process.env,
      GLYPHDECK_HOST: "127.0.0.1",
      GLYPHDECK_PORT: String(port),
      GLYPHDECK_DATA_DIR: DATA_DIR,
      GLYPHDECK_ADMIN_PASSWORD: PASSWORD,
    },
  });
  fs.writeFileSync(assertValidationPath(path.join(PID_DIR, "backend.pid")), String(backend.pid));
  fs.writeFileSync(assertValidationPath(path.join(PID_DIR, "backend-port.txt")), String(port));
  await waitForHealth(baseUrl);
  record("isolated backend bootstrapped through GLYPHDECK_ADMIN_PASSWORD");

  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
  await page.goto(baseUrl, { waitUntil: "networkidle" });

  await page.getByTestId("setup-screen").waitFor({ state: "detached", timeout: 10000 }).catch(() => {});
  await ensureAbsent(page, "setup-screen", "bootstrap bypassed first-run setup");
  await page.getByTestId("login-screen").waitFor({ state: "visible", timeout: 10000 });
  record("login screen visible after bootstrap");
  await page.getByTestId("login-password-input").fill(PASSWORD);
  await page.getByTestId("login-submit-button").click();
  await page.getByTestId("app-shell").waitFor({ state: "visible", timeout: 10000 });
  await ensureAbsent(page, "login-screen", "login completed");

  await page.getByTestId("activity-servers-button").click();
  await ensureServersPanel(page, "Servers rail navigation");

  // 1. Empty Servers / Sandboxes view.
  await page.getByText("No servers configured.").waitFor({ state: "visible" });
  record("empty Servers / Sandboxes state asserted");
  await capture(page, "01-empty-servers.png", "Empty Servers / Sandboxes view");

  // 2. Add SSH target form.
  await page.getByTestId("server-add-button").click();
  await page.getByTestId("server-add-form").waitFor({ state: "visible" });
  await page.getByTestId("server-add-type").selectOption("ssh_alias");
  await page.getByTestId("server-add-ssh-alias").waitFor({ state: "visible" });
  record("SSH target form asserted");
  await capture(page, "02-add-ssh-target-form.png", "Add SSH target form");

  // 3. Form validation errors.
  await page.getByTestId("server-add-submit").click();
  await page.getByText("Name is required").waitFor({ state: "visible" });
  await page.getByText("SSH alias is required").waitFor({ state: "visible" });
  record("add-target validation errors asserted");
  await capture(page, "03-add-target-validation-errors.png", "SSH target form validation errors");

  // Create a persisted SSH config through the user-facing add form.
  await page.getByTestId("server-add-id").fill(TARGET_ID);
  await page.getByTestId("server-add-name").fill("Validation SSH Target");
  await page.getByTestId("server-add-ssh-alias").fill("validation-host");
  await page.getByTestId("server-add-submit").click();
  await page.getByTestId(`server-card-${TARGET_ID}`).waitFor({ state: "visible" });
  record("saved SSH target persisted through authenticated UI");

  // The existing backend model supports a URL for SSH targets. Set a loopback
  // URL through its authenticated update API so attach-state validation can be
  // shown without a real remote host or secret-bearing SSH configuration.
  const updateResult = await page.evaluate(async ({ id, url }) => {
    const response = await fetch(`/api/server-configs/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: "Validation SSH Target",
        type: "ssh_alias",
        url,
        sshAlias: "validation-host",
        workingDir: "",
        startCommand: "",
        stopCommand: "",
        statusCommand: "",
      }),
    });
    return response.ok;
  }, { id: TARGET_ID, url: TARGET_URL });
  if (!updateResult) fail("could not prepare authenticated SSH target URL for visual validation");
  await page.reload({ waitUntil: "networkidle" });
  await page.getByTestId("activity-servers-button").click();
  await page.getByTestId(`server-card-${TARGET_ID}`).waitFor({ state: "visible" });

  // 4. Offline target.
  await setRoute(page, `**/api/server-configs/${TARGET_ID}/check`, 200, { id: TARGET_ID, status: "offline" });
  await page.getByTestId(`check-${TARGET_ID}`).click();
  await page.getByTestId("status-dot-offline").waitFor({ state: "visible" });
  record("offline target status asserted");
  await capture(page, "04-saved-offline-ssh-target.png", "Saved offline SSH target");
  await page.unroute(`**/api/server-configs/${TARGET_ID}/check`);

  // 5. Successful SSH test.
  await setRoute(page, `**/api/server-configs/${TARGET_ID}/test-ssh`, 200, { success: true, message: "SSH connection OK" });
  await page.getByTestId(`test-ssh-${TARGET_ID}`).click();
  await page.getByTestId(`action-msg-${TARGET_ID}`).getByText("SSH connection OK").waitFor({ state: "visible" });
  record("successful SSH test state asserted with mocked transport");
  await capture(page, "05-successful-ssh-test.png", "Successful SSH test state");
  await page.unroute(`**/api/server-configs/${TARGET_ID}/test-ssh`);

  // 6. Online target ready to attach.
  await setRoute(page, `**/api/server-configs/${TARGET_ID}/detect`, 200, { status: "online", message: "OpenCode detected" });
  await page.getByTestId(`detect-${TARGET_ID}`).click();
  await page.getByTestId("status-dot-online").waitFor({ state: "visible" });
  const attach = page.getByTestId(`attach-${TARGET_ID}`);
  if (await attach.isDisabled()) fail("online target with URL did not enable Attach");
  record("online target ready to attach asserted");
  await capture(page, "06-online-target-ready-to-attach.png", "Online target ready to attach");
  await page.unroute(`**/api/server-configs/${TARGET_ID}/detect`);

  // 7. Attached target.
  await attach.click();
  await page.getByTestId("active-server-banner").waitFor({ state: "visible" });
  record("attached target state asserted");
  await capture(page, "07-active-attached-target.png", "Active attached target");

  // 8. Protected delete warning while attached.
  await page.getByTestId(`remove-server-${TARGET_ID}`).click();
  await page.getByTestId(`delete-confirm-${TARGET_ID}`).waitFor({ state: "visible" });
  await page.getByText("Attached targets should be detached first.").waitFor({ state: "visible" });
  record("attached-target delete warning asserted");
  await capture(page, "08-protected-delete-warning.png", "Protected delete warning for attached target");
  await page.getByTestId(`delete-confirm-no-${TARGET_ID}`).click();

  // Detach is separate from process lifecycle and does not stop the target.
  await page.getByTestId("detach-server-button").click();
  await page.getByTestId("no-active-server").waitFor({ state: "visible" });
  record("detach cleared active target without remote stop");

  // 9. Lifecycle error.
  await setRoute(page, `**/api/server-configs/${TARGET_ID}/check`, 200, { id: TARGET_ID, status: "offline" });
  await page.getByTestId(`check-${TARGET_ID}`).click();
  await page.getByTestId("status-dot-offline").waitFor({ state: "visible" });
  await page.unroute(`**/api/server-configs/${TARGET_ID}/check`);
  await setRoute(page, `**/api/server-configs/${TARGET_ID}/start-remote`, 400, { message: "Remote start failed. Verify the SSH alias and OpenCode command." });
  await page.getByTestId(`start-remote-${TARGET_ID}`).click();
  await page.getByTestId(`error-msg-${TARGET_ID}`).waitFor({ state: "visible" });
  record("actionable lifecycle error asserted");
  await capture(page, "09-lifecycle-error.png", "Lifecycle error state");
  await page.unroute(`**/api/server-configs/${TARGET_ID}/start-remote`);

  // 10. Narrow layout.
  await page.getByTestId(`dismiss-error-${TARGET_ID}`).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByTestId(`server-card-${TARGET_ID}`).waitFor({ state: "visible" });
  record("narrow Servers / Sandboxes layout asserted");
  await capture(page, "10-narrow-layout.png", "Narrow Servers / Sandboxes layout");
}

async function cleanup() {
  if (browser) await browser.close().catch(() => {});
  if (backend && backend.exitCode === null) {
    backend.kill("SIGTERM");
    await new Promise((resolve) => setTimeout(resolve, 500));
    if (backend.exitCode === null) backend.kill("SIGKILL");
  }
}

(async () => {
  let failure;
  try {
    await run();
    writeManifest("PASS");
    console.log("Result: PASS");
  } catch (error) {
    failure = error instanceof Error ? error.message : String(error);
    writeManifest("FAIL", failure);
    console.error(`Result: FAIL — ${failure}`);
    process.exitCode = 1;
  } finally {
    await cleanup();
  }
})();
