# Client Interaction CRM

A lightweight self-hosted CRM for managing company records and customer interaction history.

Client Interaction CRM is intentionally small: a React/Vite frontend talks directly to Supabase Auth and Postgres. It is designed for one trusted team per Supabase project, not as a multitenant SaaS.

## Overview

Companies and their interaction history are shared by the authenticated members of one small, trusted team. The browser talks directly to Supabase; there is no custom backend server.

## Tech stack

- React and Vite
- React Router
- Supabase Auth, Postgres, Data API, and RLS
- Vercel for the recommended static frontend deployment

## Features

- Email/password authentication with password recovery.
- Shared company directory with optional legal form, Tax ID, contact, address, website, and phone fields.
- Interaction journal linked to companies.
- Create, edit, and delete companies and interactions.
- "My records" filters based on database-assigned creator IDs.
- Row Level Security that blocks anonymous CRM access.
- Neutral, configurable branding with an optional custom logo.
- Guided Supabase setup and Vercel-ready SPA configuration.
- GitHub Actions CI.

## Easy install

Ordinary users do not need Git, Node.js, npm, Go, SQL knowledge, Supabase CLI knowledge, Vercel CLI knowledge, or a GitHub account.

### Windows

1. Download `ClientInteractionCRM-Setup-windows-amd64.exe` from an official GitHub Release. Download its `.sha256` file if you want to verify the download.
2. Double-click the launcher. Windows SmartScreen may warn about the unsigned first release; verify that the file came from the official repository rather than disabling SmartScreen.
3. Choose a work folder and follow the company-name and optional-logo prompts.
4. Sign in to Supabase in the browser, or create a Supabase account there if you are new.
5. Choose where the database should be hosted and let the installer prepare it.
6. Sign in to Vercel in the browser, or create a Vercel account there if you are new.
7. Create or invite the first CRM user on the Supabase page opened by the installer.
8. Open the final CRM address and sign in.

No PowerShell commands or administrator access are required for normal Windows installation.

### Linux

1. Download `client-interaction-crm-setup-linux-amd64` and its `.sha256` file from an official GitHub Release.
2. Verify the checksum if desired, then mark the launcher executable with `chmod +x client-interaction-crm-setup-linux-amd64`.
3. Run `./client-interaction-crm-setup-linux-amd64` and follow the same guided Supabase, Vercel, branding, database-location, and first-user steps shown above.
4. Open the final CRM address and sign in.

The launcher stores its application payload, optional portable Node runtime, and troubleshooting log entirely under the selected user-writable directory. It does not require administrator privileges or modify the global `PATH`. See [launcher acceptance and maintenance documentation](installer/README.md) for platform-specific instructions, cleanup, release details, and the unsigned Windows SmartScreen notice.

## Developer setup

### Requirements

- Node.js 20 or newer.
- npm.
- A Supabase account with permission/quota to use a project.
- A Vercel account for the recommended deployment path. GitHub is needed only when using the Git-integrated deployment workflow.

### 1. Create your copy

Use this repository as a GitHub template (or fork/clone it), then:

```bash
npm ci
npm run setup
```

The setup wizard can:

1. configure your organization name and optional logo;
2. sign in to the Supabase CLI;
3. use an existing Supabase project or create a new one;
4. link the project and apply the included database migration;
5. push the safe Auth configuration with public signup disabled;
6. write `.env.local` with the project URL and browser-safe publishable key;
7. write a Supabase Auth configuration for localhost and an optional Vercel URL.

The wizard never reads, writes, or stores a Supabase secret/service-role key. Create the first user through Supabase Dashboard → Authentication → Users → Invite user.

### 2. Run locally

```bash
npm run dev
```

Open the URL printed by Vite (normally `http://localhost:5173`).

After inviting a user, open the invitation while the local server or deployed site is available and choose a password.

## Branding

Default branding is deliberately neutral. If no custom logo is configured, the UI shows a simple monogram using the first letter of the configured organization/product name.

`npm run setup` can copy a PNG, JPG, WEBP, or SVG logo into `public/branding/` and update `src/config/branding.json`.

You can also edit the file manually:

```json
{
  "productName": "Client Interaction CRM",
  "organizationName": "Northstar Engineering",
  "logoPath": "branding/logo.svg"
}
```

Branding contains no secrets and is expected to be committed in a deployer's own repository.

## Architecture

```text
Browser (React + Vite)
        |
        +--> Supabase Auth
        |
        +--> Supabase Postgres / REST API
                 |
                 +--> RLS policies
```

There is no custom application server. Browser code uses only a Supabase publishable key. Authentication identifies the user, while Postgres Row Level Security is the actual data-access boundary.

## Security model

Each deployment is designed for **one trusted team**. All authenticated users in that Supabase project can read, create, edit, and delete the team's CRM records. Anonymous access to the CRM tables is explicitly revoked and there are no RLS policies for the anonymous role.

`created_by` is assigned by the database from `auth.uid()` and is preserved on edits. The "My records" filter is only a UI convenience for attribution; it is **not** tenant isolation or an ownership permission boundary.

Public signup is disabled by default. The deployer invites users through Supabase Auth. Never put a Supabase secret key, legacy `service_role` key, database password, or personal access token into any `VITE_*` variable.

The tracked Auth configuration keeps the email provider enabled so invited users can sign in, while the global `auth.enable_signup = false` setting blocks self-registration.

## Data model

The tracked migration creates two application tables:

- `clients`: company/contact information and creator attribution.
- `calls`: interaction date, description, linked company, and creator attribution. The historical table name remains `calls` to keep the application simple, while the public UI calls these records **Interactions**.

Deleting a company cascades to its interaction history. Deleting an Auth user keeps shared CRM records and clears the UUID attribution where the foreign key applies.

## Supabase setup without the wizard

If you prefer to configure everything manually:

```bash
npx supabase@latest login
npx supabase@latest link --project-ref YOUR_PROJECT_REF
npx supabase@latest db push --linked --skip-vault
npx supabase@latest config diff --project-ref YOUR_PROJECT_REF
# Review the displayed changes, then:
npx supabase@latest config push --project-ref YOUR_PROJECT_REF
cp .env.example .env.local
```

Then put the project's URL and **publishable** key into `.env.local`:

| Variable | Required | Purpose |
| --- | --- | --- |
| `VITE_SUPABASE_URL` | Yes | Your Supabase project API URL. |
| `VITE_SUPABASE_PUBLISHABLE_KEY` | Yes | Browser-safe project publishable key. |

For older Supabase projects the frontend also accepts `VITE_SUPABASE_ANON_KEY` as a compatibility fallback, but new deployments should use a publishable key.

In Supabase Auth URL Configuration, set the Site URL to your deployed application and allow `https://your-domain/**` as a redirect URL. The tracked `supabase/config.toml` starts with localhost URLs; the wizard can add an optional Vercel URL and shows a configuration diff before pushing it.

To add more team members later, use **Supabase Dashboard → Authentication → Users → Invite user**.

## Deploying with Vercel

1. Push this clean project to your GitHub repository.
2. In Vercel, choose **Add New → Project**, import the repository, and accept the Vite defaults.
3. Add `VITE_SUPABASE_URL` and `VITE_SUPABASE_PUBLISHABLE_KEY` from your local `.env.local` as Vercel environment variables for Production, Preview, and Development as appropriate.
4. Deploy. `vercel.json` rewrites direct SPA routes—including `/login`, `/clients`, `/interactions`, `/forgot-password`, and `/update-password`—to `index.html`.
5. Add the deployed production URL and, if desired, preview URL patterns to Supabase Auth URL Configuration. Re-run `npm run setup` with the production Vercel URL or update `supabase/config.toml`, review `supabase config diff`, and push only to your dedicated project.

Any host that serves Vite's `dist/` directory with an SPA fallback can be used as a secondary option; Vercel is the documented default.

## Local development

```bash
npm ci
npm run dev
npm run lint
npm run build
```

`npm run check` runs lint and production build together.

## Production build

```bash
npm run build
npm run preview
```

For database changes, add a new SQL file under `supabase/migrations/` and apply it to a disposable/test project before using it on important data.

## Screenshots

No production screenshots are stored in this repository. See `docs/screenshots/README.md` for a safe fictional-data shot list.

## Known limitations

- Single trusted team per Supabase project; no SaaS multitenancy.
- No granular roles or record-level ownership restrictions between authenticated team members.
- No advanced reporting, pipeline/deal management, email integration, or offline mode.
- User administration is intentionally delegated to Supabase Auth.
- The setup wizard depends on the current Supabase CLI and may fall back to a small number of Dashboard steps if platform permissions or CLI behavior prevent automation.

## Repository hygiene

Do not commit `.env.local`, Supabase/database dumps, production exports, customer data, logs, or credentials. The `.gitignore` contains explicit exclusions for common private artifacts.

## Roadmap

Potential later improvements include search/sorting, richer interaction types, import/export, and optional localization. They are intentionally outside the initial GitHub-ready MVP.

## License

MIT. See [LICENSE](LICENSE).

## Contributing

Small, focused contributions are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md), keep customer data and credentials out of changes, and run `npm run check` before opening a pull request.
