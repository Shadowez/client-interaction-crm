# Contributing

Thanks for considering a contribution.

Keep the project intentionally small and self-hosted. Before opening a pull request:

```bash
npm ci
npm run lint
npm run build
```

Do not include real customer/company data, credentials, Supabase/database dumps, screenshots from production systems, or proprietary branding/assets.

Database changes should be supplied as new files in `supabase/migrations/` and should preserve the documented single-trusted-team security model unless the change explicitly proposes a different model.
