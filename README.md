# Client Interaction CRM

Client Interaction CRM is a lightweight deploy-your-own web CRM for a small, trusted team. It keeps company details and customer interaction history together, with authentication and database access enforced by Supabase and a guided deployment to Vercel.

## What it does

- Manage companies, contacts, and interaction history.
- Create, edit, and delete shared CRM records.
- Attribute records to authenticated users and switch between **All** and **My** views.
- Provide email/password login and password recovery.
- Apply custom organization naming and an optional logo.
- Deploy into infrastructure controlled by the user rather than a hosted CRM account.

## Before you install

The launcher deploys your CRM using **Supabase for authentication and the database** and **Vercel for web hosting**.

You need:

- an internet connection;
- a [Supabase account](https://supabase.com/dashboard);
- a [Vercel account](https://vercel.com/signup);
- access to a web browser;
- an email address for the first CRM user.

You do **not** need Git, Node.js, npm, Go, SQL knowledge, Supabase CLI knowledge, Vercel CLI knowledge, or administrator/root privileges for normal launcher use.

## Hosting plan notes

The installer creates a **new dedicated Supabase project by default**. At the time of this release, the Supabase Free plan allows two active free projects, and paused projects do not count toward that active-project limit. If your allowance is already used, pause or delete an unused project or choose an appropriate paid plan. Free projects may also be paused for inactivity. Limits and behavior can change; review the current [Supabase pricing](https://supabase.com/pricing) and [billing documentation](https://supabase.com/docs/guides/platform/billing-on-supabase) before deploying.

Vercel's current Hobby plan is intended for personal, non-commercial use. Professional, business, or other commercial CRM use requires a plan permitted for that use, such as the appropriate Pro or business offering under the current terms. Review [Vercel pricing](https://vercel.com/pricing) and the [Vercel Terms of Service](https://vercel.com/legal/terms). This is a summary, not legal advice; service terms and pricing may change.

## Easy install

### Windows

1. Download `ClientInteractionCRM-Setup-windows-amd64.exe` from the GitHub Release. Optionally download its `.sha256` file too.
2. Double-click the executable.
3. Follow the guided console installer.
4. Sign in or create a Supabase account when the browser opens.
5. Sign in or create a Vercel account when the browser opens.
6. Choose the organization name and optional logo.
7. Let the installer create and configure the CRM.
8. Create or invite the first CRM user when instructed.
9. Open the final URL and sign in.

Version 1.1.0 is unsigned, so Windows SmartScreen may show a warning. Confirm that the file came from this repository and verify its SHA-256 checksum if desired. Do not disable SmartScreen globally. Ordinary installation requires no PowerShell or terminal commands and no administrator access.

### Linux

1. Download `client-interaction-crm-setup-linux-amd64` and optionally its `.sha256` file from the GitHub Release.
2. If desired, verify it with `sha256sum -c client-interaction-crm-setup-linux-amd64.sha256`.
3. Run `chmod +x client-interaction-crm-setup-linux-amd64`.
4. Run `./client-interaction-crm-setup-linux-amd64` and follow the guided Supabase, Vercel, branding, and first-user steps.
5. Open the final URL and sign in.

## What the installer creates

- A dedicated Supabase project by default, containing Auth configuration, Postgres tables, and Row Level Security policies.
- A Vercel web deployment of the CRM.
- Application source, an optional portable Node.js runtime, state, and troubleshooting logs under the local folder you choose.

The launcher does not install Node.js globally or modify the global `PATH`.

## Security model

- Anonymous visitors cannot access CRM records.
- Authenticated users in one trusted team share the CRM records.
- Supabase Row Level Security enforces database access.
- Public CRM registration is disabled; users are invited by the deployer.
- No service-role, secret, or elevated API key is shipped to the browser.
- Frontend configuration contains only the Supabase project URL and browser-safe publishable key.

The **My** filter is attribution, not a private-record boundary: authenticated team members can work with shared records.

## First CRM user

One manual step is intentional. When the launcher opens **Supabase Dashboard → Authentication → Users**, create or invite the first user's email address. Complete the invitation/password flow, then use that account to sign in to the CRM. Add later team members from the same Dashboard page.

## Updating or rerunning setup

The launcher stores non-secret progress in the selected installation folder. If setup is cancelled or interrupted, run the same launcher again and select the same folder; it resumes safely and avoids silently creating a duplicate Supabase project. Keep important data backed up according to your chosen hosting plans.

## Developer setup

Developer requirements are Node.js 20 or newer, npm, a Supabase account/project, and a Vercel account for the documented deployment path.

```bash
npm ci
npm run setup
npm run dev
```

The setup wizard configures branding, links or creates a Supabase project, applies the migration and Auth configuration, and writes the browser-safe `.env.local` values. It never reads, writes, or stores a Supabase service-role key.

Useful commands:

```bash
npm run lint
npm run build
npm run check
npm run preview
```

Branding can also be edited in `src/config/branding.json`. Database changes belong in new files under `supabase/migrations/` and should be tested against a disposable project first. For manual Supabase/Vercel setup and release acceptance details, see [installer/README.md](installer/README.md).

## Architecture

```text
Browser (React + Vite)
        |-- Supabase Auth
        `-- Supabase Postgres / Data API
                    `-- RLS policies
```

There is no custom application server. The recommended frontend deployment is Vercel, while Supabase provides authentication and data storage.

## Troubleshooting

- **Supabase will not create a project:** check the active Free-project allowance, pause/delete an unused project, or review paid-plan options.
- **The browser does not open:** copy the URL printed by the launcher into a browser manually.
- **The first Vercel step takes time:** initial preparation of the pinned Vercel tool can take several minutes; leave the launcher open.
- **Windows SmartScreen appears:** verify the source/checksum and use the per-file run option if you trust it; do not disable SmartScreen globally.
- **Setup was interrupted:** rerun the launcher with the same local folder to resume.
- **More detail is needed:** inspect `logs/setup.log` below the installation folder. Review it for sensitive local details before sharing.

## Known limitations

- One trusted team per Supabase project; no SaaS multitenancy or granular roles.
- No advanced reporting, deal pipeline, email integration, or offline mode.
- User administration is handled through Supabase Auth.

## Repository hygiene

Never commit `.env.local`, credentials, database dumps, production exports, customer data, or logs. The `.gitignore` excludes common private artifacts.

## License

MIT. See [LICENSE](LICENSE).

## Contributing

Small, focused contributions are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md), keep private data out of changes, and run `npm run check` before opening a pull request.
