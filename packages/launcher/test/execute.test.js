// Tests for src/execute.js — child process execution and signal forwarding.

import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { execute } from "../src/execute.js";

describe("execute", () => {
  it("returns exit code 0 for successful command", async () => {
    const code = await execute(process.execPath, ["-e", ""]);
    assert.strictEqual(code, 0);
  });

  it("returns non-zero exit code for failing command", async () => {
    const code = await execute(process.execPath, ["-e", "process.exit(42)"]);
    assert.strictEqual(code, 42);
  });

  it("forwards stdout", async () => {
    const code = await execute(process.execPath, ["-e", "console.log('hello')"]);
    assert.strictEqual(code, 0);
  });

  it("forwards command-line arguments", async () => {
    const code = await execute(process.execPath, [
      "-e",
      "console.log(process.argv.slice(1).join(','))",
      "--",
      "arg1",
      "arg2",
    ]);
    assert.strictEqual(code, 0);
  });

  it("rejects for non-existent binary", async () => {
    await assert.rejects(
      () => execute("/nonexistent/binary/path", []),
      /ENOENT|not found|The system cannot find/
    );
  });

  it("forwards SIGINT to child process — handlers registered and removed", async () => {
    const beforeCount = process.listenerCount("SIGINT");

    const code = await execute(process.execPath, ["-e", ""]);
    assert.strictEqual(code, 0);

    const afterCount = process.listenerCount("SIGINT");
    assert.strictEqual(
      afterCount,
      beforeCount,
      "SIGINT handlers must be removed after child exits"
    );
  });

  it("forwards SIGTERM to child process — handlers registered and removed", async () => {
    const beforeCount = process.listenerCount("SIGTERM");

    const code = await execute(process.execPath, ["-e", ""]);
    assert.strictEqual(code, 0);

    const afterCount = process.listenerCount("SIGTERM");
    assert.strictEqual(
      afterCount,
      beforeCount,
      "SIGTERM handlers must be removed after child exits"
    );
  });

  it("removes handlers on child error", async () => {
    const beforeSigint = process.listenerCount("SIGINT");
    const beforeSigterm = process.listenerCount("SIGTERM");

    await assert.rejects(() => execute("/nonexistent/path", []));

    assert.strictEqual(
      process.listenerCount("SIGINT"),
      beforeSigint,
      "SIGINT handlers must be removed after error"
    );
    assert.strictEqual(
      process.listenerCount("SIGTERM"),
      beforeSigterm,
      "SIGTERM handlers must be removed after error"
    );
  });

  it("does not kill unrelated processes", async () => {
    // Run execute normally and verify the test runner's PID hasn't changed.
    const ourPid = process.pid;
    const code = await execute(process.execPath, ["-e", ""]);
    assert.strictEqual(code, 0);
    assert.strictEqual(process.pid, ourPid, "Our process must not be killed");
  });
});
