// Executes the verified GlyphDeck binary as a child process.

import { spawn } from "node:child_process";

/**
 * Runs the GlyphDeck binary with forwarded arguments and stdio.
 * Returns the child's exit code.
 *
 * @param {string} binaryPath - Path to the verified binary.
 * @param {string[]} args - Command-line arguments to forward.
 * @returns {Promise<number>} The child process exit code.
 */
export function execute(binaryPath, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(binaryPath, args, {
      stdio: "inherit",
      shell: false,
      detached: false,
      env: process.env,
    });

    // Separate handlers for each signal.
    const onSigint = () => {
      if (child.pid) {
        try { child.kill("SIGINT"); } catch { /* already gone */ }
      }
    };

    const onSigterm = () => {
      if (child.pid) {
        try { child.kill("SIGTERM"); } catch { /* already gone */ }
      }
    };

    const cleanupHandlers = () => {
      process.removeListener("SIGINT", onSigint);
      process.removeListener("SIGTERM", onSigterm);
    };

    process.on("SIGINT", onSigint);
    process.on("SIGTERM", onSigterm);

    child.on("error", (err) => {
      cleanupHandlers();
      reject(err);
    });

    child.on("close", (code, signal) => {
      cleanupHandlers();

      if (signal) {
        // Child was killed by a signal — forward to self for correct shell behavior.
        process.kill(process.pid, signal);
        return;
      }
      resolve(code ?? 1);
    });
  });
}
