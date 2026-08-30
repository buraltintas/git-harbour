# Deploy the API to Koyeb Free

GitHub Pages hosts only the static web application. Koyeb builds and runs the Go API from `apps/api/Dockerfile`; Supabase remains the only database.

## Create the service

1. In Koyeb, choose **Create Web Service → GitHub** and grant access only to the `git-harbour` repository.
2. Select the production branch (`main`) and enable automatic deployment.
3. Choose **Dockerfile** as the builder.
4. Set **Work directory** to `apps/api` and **Dockerfile location** to `Dockerfile`.
5. Choose the Free instance and the Frankfurt region.
6. Expose HTTP port `8080` at route `/`.
7. Configure an HTTP health check for `GET /healthz` on port `8080`.

Koyeb supplies `PORT`; do not override the Docker entrypoint or command. The free instance scales to zero after one hour without traffic and normally wakes in a few seconds.

## Environment and secrets

Set these as plain environment values:

```text
APP_ENV=production
DB_MAX_CONNS=5
DB_MIN_CONNS=0
DB_MAX_CONN_LIFETIME=30m
WEB_ORIGIN=https://buraltintas.github.io
PUBLIC_API_URL=https://<KOYEB_DOMAIN>
WEB_APP_URL=https://buraltintas.github.io/git-harbour/#/
GITHUB_CLIENT_ID=<GitHub OAuth client ID>
GITHUB_OAUTH_CALLBACK=https://<KOYEB_DOMAIN>/auth/github/callback
GITHUB_WEB_CALLBACK=https://buraltintas.github.io/git-harbour/#/auth/callback
```

Create Koyeb secrets and reference them as environment values:

```text
DATABASE_URL=<percent-encoded Supabase Session Pooler URI>
GITHUB_CLIENT_SECRET=<GitHub OAuth client secret>
```

Never set `GITHARBOUR_DEV_AUTH` in production. Never expose either secret as `VITE_*` or as a GitHub Pages variable.

## Connect OAuth and Pages

After Koyeb assigns the public domain:

1. Update `PUBLIC_API_URL` and `GITHUB_OAUTH_CALLBACK` in Koyeb.
2. In the GitHub OAuth App, set the homepage to `https://buraltintas.github.io/git-harbour/` and callback to `https://<KOYEB_DOMAIN>/auth/github/callback`.
3. In the GitHub repository, create the Actions variable `API_URL=https://<KOYEB_DOMAIN>`.
4. Redeploy the Koyeb service, then rerun the GitHub Pages workflow.

## Migrations

Migrations do not run during API startup. Apply them from a trusted machine before deploying a schema-dependent release:

```bash
cd apps/api
DATABASE_URL='<Session Pooler URI>' go run ./cmd/migrate up
```

Production startup intentionally fails if the database is unreachable or migrations are missing.
