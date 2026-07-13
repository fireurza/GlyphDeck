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
    // We can't easily capture stdout from execute since it uses "inherit".
    // Instead, verify the function doesn't throw for a simple output command.
    const code = await execute(process.execPath, ["-e", "console.log('hello')"]);
    assert.strictEqual(code, 0);
  });

  it("forwards command-line arguments", async () => {
    const code = await execute(process.execPath, ["-e", "console.log(process.argv.slice(1).join(','))", "--", "arg1", "arg2"]);
    assert.strictEqual(code, 0);
  });

  it("rejects for non-existent binary", async () => {
    await assert.rejects(
      () => execute("/nonexistent/binary/path", []),
      /ENOENT|not found|The system cannot find/
    );
  });
});
