# Client Interaction CRM

Client Interaction CRM is a lightweight, self-hosted CRM for one trusted small team. It keeps company details and interaction history in one shared workspace, with Supabase providing the database and authentication and Vercel hosting the web app.

It includes:

- company and contact management;
- a shared interaction history;
- **All records** and attribution-based **My records** views;
- email/password sign-in and password recovery;
- company name and logo branding;
- a guided Windows installer and a Linux launcher;
- an MIT license.

This is not a hosted SaaS or an enterprise CRM. One deployment and one Supabase project are intended for one trusted team.

## Install on Windows

1. Open the [v1.1.0 GitHub Release](https://github.com/Shadowez/client-interaction-crm/releases/tag/v1.1.0).
2. Download `ClientInteractionCRM-Setup-windows-amd64.exe`.
3. Double-click the downloaded file.
4. Follow the browser sign-in steps for Supabase and Vercel.
5. Let the installer configure and deploy the CRM.
6. Use the final CRM URL shown by the installer. It is also saved in `CRM-INFO.txt`, added as a desktop shortcut, and opened in your browser.

You do **not** need to install Git, Node.js, npm, Go, Supabase CLI, Vercel CLI, PowerShell, or SQL tools. The launcher manages the components it needs and does not require administrator rights.

Version 1.1.0 is unsigned, so Windows SmartScreen may display a warning. Check that the file came from this repository and compare its SHA-256 checksum with the sidecar file in the Release. Do not disable SmartScreen globally.

## Install on Linux

Download `client-interaction-crm-setup-linux-amd64` and its checksum from the Release, then run:

```bash
sha256sum -c client-interaction-crm-setup-linux-amd64.sha256
chmod +x client-interaction-crm-setup-linux-amd64
./client-interaction-crm-setup-linux-amd64
```

Follow the same guided Supabase, Vercel, branding, and first-user steps.

## What setup creates

- A dedicated Supabase project by default, with Auth, Postgres tables, and Row Level Security policies.
- A Vercel deployment of the CRM web interface.
- Local application files, non-secret resume state, troubleshooting logs, and—only when necessary—a CRM-owned portable runtime and npm cache.
- On Windows, `CRM-INFO.txt`, a desktop shortcut, an ownership manifest, and `Uninstall Client Interaction CRM.exe`.

The launcher never installs Node.js globally or changes the global `PATH` or npm configuration.

## Uninstall on Windows

Run the installed `Uninstall Client Interaction CRM.exe`. Before changing anything, it shows the exact local paths selected for cleanup and asks for final confirmation, with **No** as the default.

The uninstaller can remove the CRM-owned installation, portable runtime, npm cache, logs, state, AppData directories, `CRM-INFO.txt`, shortcuts, and the uninstaller itself. It never treats a system Node installation or the user's shared npm cache as CRM-owned.

Local uninstall does **not** delete the Supabase project/database or Vercel project/deployment. Supabase and Vercel CLI sign-out are separate options, are **off by default**, and may affect other projects using the same login. Delete online infrastructure manually only if you also intend to permanently remove the hosted CRM and its data.

## Data and security model

- Supabase Auth protects access to the CRM.
- Anonymous users have no access to CRM records.
- Authenticated members of the same deployment/team share records.
- **My records** is an attribution filter, not an authorization or private-record boundary.
- Row Level Security is enabled for CRM tables.
- Public signup is disabled by deployment configuration; the deployer creates or invites users through Supabase Auth.
- The browser receives only the Supabase project URL and browser-safe publishable key—not a service-role or secret key.
- One Supabase project represents one trusted team; the application does not provide SaaS multitenancy or enterprise RBAC.

## Accounts, hosting, and cost

Installation requires an internet connection, a browser, and Supabase and Vercel accounts.

Supabase project allowances, inactivity behavior, and pricing can change. Check the current [Supabase pricing](https://supabase.com/pricing) and [billing documentation](https://supabase.com/docs/guides/platform/billing-on-supabase) before deploying.

Vercel plan eligibility and permitted use can also change. Review the current [Vercel pricing](https://vercel.com/pricing), [Hobby plan documentation](https://vercel.com/docs/plans/hobby), and [Terms of Service](https://vercel.com/legal/terms) before using the CRM for commercial work.

## First CRM user

When setup opens **Supabase Dashboard → Authentication → Users**, create or invite the first user. Complete the email/password flow, then sign in to the CRM. Add later team members from the same Supabase page.

## Resume and troubleshooting

If setup is interrupted, run the same launcher again and choose the same local directory. It stores non-secret progress and resumes without silently creating another Supabase project or Vercel deployment.

Troubleshooting logs are stored under the chosen installation directory in `logs/setup.log`. Review logs for sensitive local details before sharing them.

## Development

Development requires Node.js 20 or newer and npm:

```bash
npm ci
npm run check
npm run dev
```

Database changes belong in new files under `supabase/migrations/` and should be tested against a disposable project. Maintainer launcher and release instructions are documented in [installer/README.md](installer/README.md); `Run-Windows-Setup.cmd` is a maintainer convenience, not the public installation path.

## Limitations

- One trusted team per Supabase project.
- No granular roles, SaaS multitenancy, advanced reporting, deal pipeline, email integration, or offline mode.
- User administration is handled through Supabase Auth.

## License

Client Interaction CRM is available under the [MIT License](LICENSE).

## Contributing

Focused contributions are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md), never commit credentials or customer data, and run `npm run check` before opening a pull request.
