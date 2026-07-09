#!/usr/bin/env node
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { execFile } from "child_process";
import { promisify } from "util";
import path from "path";
import { fileURLToPath } from "url";

const execFileAsync = promisify(execFile);
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const REPO_ROOT = path.resolve(__dirname, "../../");

const server = new Server(
  {
    name: "rolloutguardian-mcp-adapter",
    version: "0.1.0",
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [
      {
        name: "rolloutguardian_evaluate",
        description:
          "Given a feature flag and a proposed rollout percentage change, returns the full resilience-aware decision object (allow/warn/block) and reasons based on Chaos coverage and SRM error budgets.",
        inputSchema: {
          type: "object",
          properties: {
            flag_key: {
              type: "string",
              description: "The feature flag key (e.g., checkout-v2-express-pay)",
            },
            target_rollout_pct: {
              type: "number",
              description: "Proposed target rollout percentage (0 to 100)",
            },
          },
          required: ["flag_key", "target_rollout_pct"],
        },
      },
      {
        name: "rolloutguardian_explain",
        description:
          "Returns the human-readable audit reasoning and signal breakdown for a feature flag's downstream blast radius.",
        inputSchema: {
          type: "object",
          properties: {
            flag_key: {
              type: "string",
              description: "The feature flag key to explain",
            },
          },
          required: ["flag_key"],
        },
      },
    ],
  };
});

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  if (name === "rolloutguardian_evaluate") {
    const flagKey = String(args?.flag_key || "checkout-v2-express-pay");
    const targetPct = Number(args?.target_rollout_pct || 50);

    try {
      // Execute `go run ./cmd/rolloutguardian evaluate --flag <key> --target-rollout <pct> --json --dry-run`
      const { stdout } = await execFileAsync(
        "go",
        [
          "run",
          "./cmd/rolloutguardian",
          "evaluate",
          "--flag",
          flagKey,
          "--target-rollout",
          String(targetPct),
          "--json",
          "--dry-run",
        ],
        { cwd: REPO_ROOT }
      );

      return {
        content: [
          {
            type: "text",
            text: stdout,
          },
        ],
      };
    } catch (error: any) {
      return {
        isError: true,
        content: [
          {
            type: "text",
            text: `Error evaluating rollout decision via CLI: ${error.message || error}`,
          },
        ],
      };
    }
  }

  if (name === "rolloutguardian_explain") {
    const flagKey = String(args?.flag_key || "checkout-v2-express-pay");

    try {
      const { stdout } = await execFileAsync(
        "go",
        ["run", "./cmd/rolloutguardian", "explain", "--flag", flagKey],
        { cwd: REPO_ROOT }
      );

      return {
        content: [
          {
            type: "text",
            text: stdout,
          },
        ],
      };
    } catch (error: any) {
      return {
        isError: true,
        content: [
          {
            type: "text",
            text: `Error explaining flag reasoning via CLI: ${error.message || error}`,
          },
        ],
      };
    }
  }

  throw new Error(`Unknown tool: ${name}`);
});

async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("RolloutGuardian MCP Adapter running on stdio");
}

main().catch((err) => {
  console.error("Fatal error starting MCP adapter:", err);
  process.exit(1);
});
