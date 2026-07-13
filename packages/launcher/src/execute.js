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

    // Forward signals to the child.
    const forwardSignal = (signal) => {
      if (child.pid) {
        try {
          child.kill(signal);
        } catch {
          // Child may already be gone.
        }
      }
    };

    process.on("SIGINT", forwardSignal);
    process.on("SIGTERM", forwardSignal);

    child.on("error", (err) => {
      reject(err);
    });

    child.on("close", (code, signal) => {
      // Clean up signal handlers.
      process.removeListener("SIGINT", forwardSignal);
      process.removeListener("SIGTERM", forwardSignal);

      if (signal) {
        // Forward the signal to ourselves so the exit status reflects it.
        process.kill(process.pid, signal);
        return;
      }
      resolve(code ?? 1);
    });
  });
}
