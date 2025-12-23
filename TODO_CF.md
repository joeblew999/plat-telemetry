# TODO: Cloudflare Migration

## Overview

Migrate binary releases from GitHub Releases to Cloudflare R2 for faster uploads and downloads. Hugo docs remain on Cloudflare Pages.

## Current State

| Component | Current Host | Target |
|-----------|--------------|--------|
| Hugo docs | GitHub Pages | Cloudflare Pages (already works) |
| Binary releases | GitHub Releases | Cloudflare R2 |
| Taskfiles | GitHub Pages | Stay on GitHub Pages |

## Why Cloudflare R2?

- **Faster uploads**: GitHub release uploads are slow (especially for large binaries like telegraf at 150MB)
- **Faster downloads**: R2 edge caching provides better download speeds globally
- **No egress fees**: R2 has free egress, GitHub has limits
- **Unified platform**: Already using Cloudflare Pages for docs
- **S3 compatible**: Works with existing tooling

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    CLOUDFLARE INFRASTRUCTURE                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Cloudflare Pages (docs)                                        │
│  └── joeblew999.github.io/plat-telemetry                       │
│      └── Hugo static site                                       │
│      └── Taskfiles for remote include                           │
│                                                                  │
│  Cloudflare R2 (binaries)                                       │
│  └── plat-telemetry-releases bucket                            │
│      └── arc-main/                                              │
│          └── arc-darwin-arm64.tar.gz                           │
│          └── arc-linux-amd64.tar.gz                            │
│      └── nats-v2.10.24/                                        │
│          └── nats-server-darwin-arm64.tar.gz                   │
│          └── nats-server-linux-amd64.tar.gz                    │
│      └── ...                                                    │
│                                                                  │
│  Public URL: https://releases.plat-telemetry.dev               │
│  (or: https://r2.joeblew999.workers.dev)                       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Implementation Plan

### Phase 1: R2 Bucket Setup

1. **Create R2 bucket** in Cloudflare dashboard
   - Bucket name: `plat-telemetry-releases`
   - Public access: Yes (for downloads)
   - CORS: Allow all origins

2. **Configure custom domain** (optional)
   - `releases.plat-telemetry.dev` → R2 bucket
   - Or use workers.dev subdomain

3. **Generate API tokens**
   - Create R2 API token with read/write permissions
   - Store as secrets: `CF_ACCOUNT_ID`, `CF_R2_ACCESS_KEY_ID`, `CF_R2_SECRET_ACCESS_KEY`

### Phase 2: Update bin:download Tasks

Change download URLs to point to R2:

```yaml
# Before (GitHub Releases)
vars:
  NATS_RELEASE_URL: https://github.com/{{.RELEASE_REPO}}/releases/download/{{.RELEASE_VERSION}}

# After (Cloudflare R2)
vars:
  NATS_RELEASE_URL: https://releases.plat-telemetry.dev/nats-{{.NATS_VERSION}}
```

Download command stays the same (just different URL):
```yaml
bin:download:
  cmds:
    - curl -L {{.NATS_RELEASE_URL}}/{{.NATS_BIN_NAME}}-{{.GOOS}}-{{.GOARCH}}.tar.gz | tar xz -C {{.NATS_BIN}}
```

### Phase 3: Update release:binary Task

Replace `gh release upload` with R2 upload:

```yaml
release:binary:
  desc: "Release subsystem binary to R2 (usage: task release:binary SUBSYSTEM=nats)"
  vars:
    SUBSYSTEM: '{{.SUBSYSTEM}}'
    R2_BUCKET: plat-telemetry-releases
    R2_ENDPOINT: https://{{.CF_ACCOUNT_ID}}.r2.cloudflarestorage.com
  cmds:
    - task: '{{.SUBSYSTEM}}:package'
    - |
      VERSION=$(task {{.SUBSYSTEM}}:config:version 2>/dev/null || echo "latest")

      # Upload to R2 using aws s3 CLI (S3-compatible)
      aws s3 cp {{.DIST_DIR}}/{{.SUBSYSTEM}}*-{{OS}}-{{ARCH}}.tar.gz \
        s3://{{.R2_BUCKET}}/{{.SUBSYSTEM}}-${VERSION}/ \
        --endpoint-url {{.R2_ENDPOINT}}
```

### Phase 4: Add rclone or aws CLI

Add tooling for R2 uploads:

```yaml
# In root Taskfile.yml
cf:
  desc: Cloudflare R2 operations
  vars:
    R2_BUCKET: plat-telemetry-releases
    R2_ENDPOINT: https://{{.CF_ACCOUNT_ID}}.r2.cloudflarestorage.com

cf:install:
  desc: Install AWS CLI for R2 access
  cmds:
    - |
      if ! command -v aws &> /dev/null; then
        if [ "$(uname)" = "Darwin" ]; then
          brew install awscli
        else
          curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
          unzip awscliv2.zip && sudo ./aws/install
        fi
      fi
  status:
    - command -v aws

cf:configure:
  desc: Configure AWS CLI for R2
  cmds:
    - |
      aws configure set aws_access_key_id $CF_R2_ACCESS_KEY_ID
      aws configure set aws_secret_access_key $CF_R2_SECRET_ACCESS_KEY
      aws configure set default.region auto
```

### Phase 5: Update GitHub Actions

For CI, use wrangler or aws CLI with R2:

```yaml
# .github/workflows/ci.yml
- name: Install wrangler
  run: npm install -g wrangler

- name: Upload to R2
  run: task ci:release
  env:
    CLOUDFLARE_API_TOKEN: ${{ secrets.CLOUDFLARE_API_TOKEN }}
    CF_ACCOUNT_ID: ${{ secrets.CF_ACCOUNT_ID }}
    CF_R2_ACCESS_KEY_ID: ${{ secrets.CF_R2_ACCESS_KEY_ID }}
    CF_R2_SECRET_ACCESS_KEY: ${{ secrets.CF_R2_SECRET_ACCESS_KEY }}
```

## URL Structure

```
# Per-subsystem versioned releases
https://releases.plat-telemetry.dev/nats-v2.10.24/nats-server-darwin-arm64.tar.gz
https://releases.plat-telemetry.dev/arc-main/arc-linux-amd64.tar.gz
https://releases.plat-telemetry.dev/telegraf-master/telegraf-darwin-arm64.tar.gz

# Latest symlinks (optional - via Workers)
https://releases.plat-telemetry.dev/nats-latest/nats-server-darwin-arm64.tar.gz
```

## Migration Steps

1. [ ] Create R2 bucket in Cloudflare dashboard
2. [ ] Configure public access and CORS
3. [ ] Generate R2 API tokens
4. [ ] Add tokens to GitHub secrets
5. [ ] Add `cf:` tasks to root Taskfile
6. [ ] Update all `*_RELEASE_URL` variables to use R2
7. [ ] Update `release:binary` task to upload to R2
8. [ ] Update CI workflow with Cloudflare secrets
9. [ ] Test local upload: `task release:binary SUBSYSTEM=nats`
10. [ ] Test local download: `task nats:clean && task nats:ensure`
11. [ ] Push and verify CI works
12. [ ] (Optional) Create "latest" symlinks with Cloudflare Workers

## Benefits After Migration

| Metric | GitHub Releases | Cloudflare R2 |
|--------|----------------|---------------|
| Upload speed | Slow (150MB = 5+ min) | Fast (< 30s) |
| Download speed | Good | Better (edge cached) |
| Egress cost | Limited free | Free |
| Reliability | Good | Excellent |
| Global reach | Yes | Yes (more PoPs) |

## Rollback Plan

If R2 migration fails:
1. Keep GitHub Release URLs as fallback
2. `bin:download` can try R2 first, fall back to GitHub:
   ```yaml
   cmds:
     - curl -L {{.R2_URL}}/... || curl -L {{.GH_URL}}/...
   ```

## Related

- [Cloudflare R2 Docs](https://developers.cloudflare.com/r2/)
- [R2 S3 Compatibility](https://developers.cloudflare.com/r2/api/s3/)
- [Wrangler CLI](https://developers.cloudflare.com/workers/wrangler/)
