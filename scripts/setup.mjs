#!/usr/bin/env node
import { copyFile, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { extname, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { createInterface } from "node:readline/promises";
import { stdin as input, stdout as output } from "node:process";

const root = process.cwd();
const rl = createInterface({ input, output });
const npx = process.platform === "win32" ? "npx.cmd" : "npx";

const log = (message = "") => console.log(message);
const note = (message) => console.log(`  ${message}`);
const heading = (message) => console.log(`\n${message}\n${"-".repeat(message.length)}`);

const ask = async (question, fallback = "") => {
  const suffix = fallback ? ` [${fallback}]` : "";
  const answer = (await rl.question(`${question}${suffix}: `)).trim();
  return answer || fallback;
};

const askYesNo = async (question, defaultYes = true) => {
  const hint = defaultYes ? "Y/n" : "y/N";
  const answer = (await rl.question(`${question} (${hint}): `)).trim().toLowerCase();
  if (!answer) return defaultYes;
  return answer === "y" || answer === "yes";
};

const run = (command, args, { capture = false, allowFailure = false } = {}) => {
  const result = spawnSync(command, args, {
    cwd: root,
    encoding: "utf8",
    stdio: capture ? ["inherit", "pipe", "pipe"] : "inherit",
    env: process.env,
  });

  if (result.error && !allowFailure) throw result.error;
  if (result.status !== 0 && !allowFailure) {
    throw new Error(`${command} ${args.join(" ")} failed with exit code ${result.status}.`);
  }
  return result;
};

const runSupabase = (args, options = {}) =>
  run(npx, ["--yes", "supabase@latest", "--agent", "no", ...args], options);

const parseJson = (text) => {
  if (!text?.trim()) return null;
  const startArray = text.indexOf("[");
  const startObject = text.indexOf("{");
  const starts = [startArray, startObject].filter((value) => value >= 0);
  if (!starts.length) return null;
  const start = Math.min(...starts);
  try {
    return JSON.parse(text.slice(start));
  } catch {
    return null;
  }
};

const getProjects = () => {
  const result = runSupabase(["projects", "list", "-o", "json"], {
    capture: true,
    allowFailure: true,
  });
  if (result.status !== 0) return null;
  const parsed = parseJson(result.stdout);
  if (Array.isArray(parsed)) return parsed;
  if (Array.isArray(parsed?.projects)) return parsed.projects;
  return [];
};

const chooseProject = async (projects) => {
  if (!projects.length) return null;
  log("\nAvailable Supabase projects:");
  projects.forEach((project, index) => {
    note(`${index + 1}. ${project.name || project.ref || project.id} (${project.region || "region unknown"})`);
  });
  note(`${projects.length + 1}. Create a new project`);

  while (true) {
    const raw = await ask("Choose a project", String(projects.length + 1));
    const choice = Number(raw);
    if (Number.isInteger(choice) && choice >= 1 && choice <= projects.length + 1) {
      return choice === projects.length + 1 ? null : projects[choice - 1];
    }
    note("Enter one of the numbers shown above.");
  }
};

const normalizeBaseUrl = (url) => (url.endsWith("/") ? url : `${url}/`);

const configureBranding = async () => {
  heading("Branding");
  const configPath = resolve(root, "src/config/branding.json");
  let current = {
    productName: "Client Interaction CRM",
    organizationName: "",
    logoPath: "",
  };

  try {
    current = { ...current, ...JSON.parse(await readFile(configPath, "utf8")) };
  } catch {
    // Use defaults if the file does not exist or is malformed.
  }

  const organizationInput = await ask(
    "Organization name (optional; '-' for neutral, blank to keep current)",
    current.organizationName || ""
  );
  const organizationName = organizationInput === "-" ? "" : organizationInput;
  const logoInput = await ask(
    "Logo file path (optional; '-' for monogram, blank to keep current/default)",
    ""
  );

  let logoPath = current.logoPath || "";
  if (logoInput === "-") {
    await mkdir(resolve(root, "public/branding"), { recursive: true });
    for (const candidate of [".png", ".jpg", ".jpeg", ".webp", ".svg"]) {
      await rm(resolve(root, `public/branding/logo${candidate}`), { force: true });
    }
    logoPath = "";
  } else if (logoInput) {
    const source = resolve(root, logoInput);
    if (!existsSync(source)) throw new Error(`Logo file not found: ${source}`);

    const extension = extname(source).toLowerCase();
    const allowed = new Set([".png", ".jpg", ".jpeg", ".webp", ".svg"]);
    if (!allowed.has(extension)) {
      throw new Error("Logo must be PNG, JPG, JPEG, WEBP, or SVG.");
    }

    await mkdir(resolve(root, "public/branding"), { recursive: true });
    for (const candidate of [".png", ".jpg", ".jpeg", ".webp", ".svg"]) {
      await rm(resolve(root, `public/branding/logo${candidate}`), { force: true });
    }
    const destination = resolve(root, `public/branding/logo${extension}`);
    await copyFile(source, destination);
    logoPath = `branding/logo${extension}`;
  }

  await writeFile(
    configPath,
    `${JSON.stringify(
      {
        productName: current.productName || "Client Interaction CRM",
        organizationName,
        logoPath,
      },
      null,
      2
    )}\n`,
    "utf8"
  );

  note(logoPath ? "Custom logo configured." : "Using the default monogram logo.");
  return { organizationName, logoPath };
};

const writeAuthConfig = async (siteUrl) => {
  const normalizedSiteUrl = normalizeBaseUrl(siteUrl || "http://localhost:5173/");
  const redirects = [
    "http://localhost:5173/**",
    "http://127.0.0.1:5173/**",
  ];
  if (!normalizedSiteUrl.startsWith("http://localhost") && !normalizedSiteUrl.startsWith("http://127.0.0.1")) {
    redirects.push(`${normalizedSiteUrl}**`);
  }

  const config = `project_id = "client-interaction-crm"\n\n[auth]\nenabled = true\nsite_url = "${normalizedSiteUrl}"\nadditional_redirect_urls = [\n${redirects
    .map((url) => `  "${url}"`)
    .join(",\n")}\n]\nenable_signup = false\n\n[auth.email]\nenable_signup = true\ndouble_confirm_changes = true\nenable_confirmations = false\nsecure_password_change = true\n`;

  await writeFile(resolve(root, "supabase/config.toml"), config, "utf8");
  return normalizedSiteUrl;
};

const collectKeyCandidates = (node, path = [], result = []) => {
  if (Array.isArray(node)) {
    node.forEach((value, index) => collectKeyCandidates(value, [...path, String(index)], result));
    return result;
  }
  if (node && typeof node === "object") {
    for (const [key, value] of Object.entries(node)) {
      collectKeyCandidates(value, [...path, key], result);
    }
    return result;
  }
  if (typeof node === "string") result.push({ value: node, path: path.join(".").toLowerCase() });
  return result;
};

const collectApiKeyRecords = (node, result = []) => {
  if (Array.isArray(node)) {
    node.forEach((value) => collectApiKeyRecords(value, result));
    return result;
  }
  if (!node || typeof node !== "object") return result;

  const value = [node.api_key, node.key, node.value].find((item) => typeof item === "string");
  if (value) {
    const label = [node.name, node.type, node.role, node.description]
      .filter((item) => typeof item === "string")
      .join(" ")
      .toLowerCase();
    result.push({ value, label });
  }

  Object.values(node).forEach((child) => collectApiKeyRecords(child, result));
  return result;
};

const findPublishableKey = (payload) => {
  const records = collectApiKeyRecords(payload);
  const candidates = collectKeyCandidates(payload);

  const publishable =
    records.find((item) => item.value.startsWith("sb_publishable_"))?.value ||
    records.find((item) => item.label.includes("publishable"))?.value ||
    records.find((item) => item.label.includes("anon") && item.value.startsWith("eyJ"))?.value ||
    candidates.find((item) => item.value.startsWith("sb_publishable_"))?.value ||
    candidates.find((item) => item.path.includes("anon") && item.value.startsWith("eyJ"))?.value ||
    null;

  return publishable;
};

const writeLocalEnv = async (url, publishableKey) => {
  const content = `# Generated by npm run setup\nVITE_SUPABASE_URL=${url}\nVITE_SUPABASE_PUBLISHABLE_KEY=${publishableKey}\n`;
  await writeFile(resolve(root, ".env.local"), content, { encoding: "utf8", mode: 0o600 });
};

const main = async () => {
  log("Client Interaction CRM setup");
  log("This wizard configures branding, a dedicated Supabase project, database migrations, and Auth.");

  const branding = await configureBranding();

  const deploymentUrl = await ask("Deployed Vercel URL (optional; e.g. https://your-app.vercel.app)");
  await writeAuthConfig(deploymentUrl || "http://localhost:5173/");

  heading("Supabase");
  let projects = getProjects();
  if (projects === null) {
    note("Supabase CLI is not logged in. Starting Supabase login…");
    runSupabase(["login"]);
    projects = getProjects();
    if (projects === null) throw new Error("Supabase login did not complete successfully.");
  }

  let project = await chooseProject(projects);
  if (project) {
    note("Important: setup will apply migrations and Auth configuration to the selected project.");
    const dedicated = await askYesNo(
      "Is this a dedicated CRM project that is safe to modify?",
      false
    );
    if (!dedicated) {
      throw new Error("Choose or create a dedicated Supabase project before continuing.");
    }
  }

  if (!project) {
    const suggestedName = branding.organizationName
      ? `${branding.organizationName} CRM`
      : "Client Interaction CRM";
    const projectName = await ask("New Supabase project name", suggestedName);
    note("The official Supabase CLI will ask for the organization, region, and database password.\n");
    runSupabase(["projects", "create", projectName]);

    const refreshed = getProjects() || [];
    const matches = refreshed
      .filter((candidate) => candidate.name === projectName)
      .sort((a, b) => String(b.created_at || "").localeCompare(String(a.created_at || "")));
    project = matches[0] || (await chooseProject(refreshed));
  }

  const projectRef = project?.ref || project?.id;
  if (!projectRef) throw new Error("Could not determine the Supabase project reference.");

  note(`Using Supabase project: ${project.name || projectRef}`);
  runSupabase(["link", "--project-ref", projectRef]);
  runSupabase(["db", "push", "--linked", "--skip-vault"]);
  runSupabase(["config", "diff", "--project-ref", projectRef], { allowFailure: true });
  if (await askYesNo("Apply the displayed Auth URL and disabled-signup configuration?", true)) {
    runSupabase(["config", "push", "--project-ref", projectRef]);
  } else {
    note("Auth configuration was not pushed. Follow the README manual steps.");
  }

  const apiKeysResult = runSupabase(
    ["projects", "api-keys", "--project-ref", projectRef, "-o", "json"],
    { capture: true }
  );
  const keysPayload = parseJson(apiKeysResult.stdout);
  const publishable = findPublishableKey(keysPayload);
  if (!publishable) {
    throw new Error(
      "Supabase CLI did not return a browser publishable/anon key. Copy the project's publishable key into .env.local manually."
    );
  }

  const supabaseUrl = `https://${projectRef}.supabase.co`;
  await writeLocalEnv(supabaseUrl, publishable);
  note("Wrote .env.local with the Supabase URL and browser-safe publishable key.");

  heading("Done");
  note("Local CRM: run `npm run dev`.");
  note("Invite the first user in Supabase Dashboard → Authentication → Users.");
  note("Deploy the frontend with Vercel, then add that URL to Supabase Auth URL Configuration if needed.");
  note("Never add .env.local, database dumps, or Supabase secret keys to Git.");
};

main()
  .catch((error) => {
    console.error(`\nSetup stopped: ${error.message}`);
    process.exitCode = 1;
  })
  .finally(() => rl.close());
