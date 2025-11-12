# CI/CD and Automation Strategy for Tokligence Gateway

**Document Version**: 1.0
**Last Updated**: 2025-11-13
**Target Version**: v0.4.0+

---

## 📊 Current Project Status

### Strengths
✅ **Comprehensive Test Suite**: 53 test suites, 99+ test cases, 91% coverage
✅ **Complete Build System**: Makefile + Go + Docker
✅ **Multi-Platform Support**: Linux/macOS/Windows (amd64, arm64)
✅ **Backend + Frontend Testing**: Go tests + Frontend tests (Vitest)
✅ **Well-Documented Testing**: Complete test reports in `docs/testing/`
✅ **Test Pass Rate**: 100% (all tests passing)

### Current Gaps
❌ **No CI/CD Configuration** - Missing `.github/workflows/`
❌ **Manual Test Execution** - Relies on `./tests/run_all_tests.sh`
❌ **No Automated Release Process**
❌ **No Automated Code Quality Checks**
❌ **No Performance Regression Tracking**

---

## 🎯 GitHub Actions CI/CD Recommendations

For **GitHub Free Organization** tier, optimized for your project's requirements.

### Why GitHub Actions?

| Feature | GitHub Actions | GitLab CI/CD | CircleCI | Jenkins |
|---------|----------------|--------------|----------|---------|
| **Cost (Free Tier)** | 2,000 min/month | 400 min/month | 6,000 min/month | Self-hosted (free) |
| **Integration** | ✅ Native GitHub | Requires GitLab | External service | Self-hosted |
| **Setup Complexity** | ⭐ Easy | ⭐⭐ Medium | ⭐⭐ Medium | ⭐⭐⭐ Hard |
| **Marketplace** | ✅ 20,000+ actions | Limited | Limited | Plugins |
| **Multi-arch builds** | ✅ Native support | ✅ Yes | ✅ Yes | Manual setup |
| **Secrets management** | ✅ Built-in | ✅ Built-in | ✅ Built-in | Manual |
| **Matrix builds** | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |
| **Caching** | ✅ Built-in | ✅ Built-in | ✅ Built-in | Manual |
| **Best for** | GitHub projects | GitLab projects | Multi-platform | Enterprise |

**Verdict**: GitHub Actions is the natural choice since your project is on GitHub, provides generous free tier, and has the richest ecosystem of pre-built actions.

### Summary of Recommended Workflows

| Workflow | File | Priority | Complexity | Estimated Time | Benefits |
|----------|------|----------|------------|----------------|----------|
| **Core CI** | `ci.yml` | 🔴 Critical | Medium | 12-18 min/run | Automated testing on every PR |
| **Dependabot** | `dependabot.yml` | 🔴 Critical | Low | N/A | Auto dependency updates |
| **Release** | `release.yml` | 🟡 High | Medium | 20-30 min/run | Multi-platform releases |
| **Docker** | `docker.yml` | 🟡 High | Medium | 15-25 min/run | Container image automation |
| **Code Quality** | `quality.yml` | 🟢 Medium | Low | 5-8 min/run | Linting and quality checks |
| **CodeQL** | `codeql.yml` | 🟢 Medium | Low | 10-15 min/run | Security vulnerability detection |
| **CLA Assistant** | `cla.yml` | 🟢 Medium | Low | <1 min/run | Contributor license management |
| **Performance** | `performance.yml` | ⚪ Low | High | 20-30 min/run | Regression detection |

**Total estimated monthly usage (conservative)**:
- Phase 1 only: ~640 minutes/month
- Phase 1-3 complete: ~950 minutes/month
- Well within GitHub Free tier (2,000 minutes/month)

---

## 1️⃣ Core CI Workflow (HIGHEST PRIORITY)

**File**: `.github/workflows/ci.yml`

### Trigger Conditions
- Push to `main`/`master` branch
- Pull Requests to `main`/`master`
- Manual workflow dispatch

### Simplified Version (Recommended to Start)

This version includes only the essential tests without complex integration setup:

```yaml
name: CI

on:
  push:
    branches: [ main, master ]
  pull_request:
    branches: [ main, master ]
  workflow_dispatch:

jobs:
  # Job 1: Backend Go Tests
  backend-tests:
    name: Backend Tests (Go 1.24)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Run Go tests
        run: make be-test

      - name: Generate coverage report
        run: go test -coverprofile=coverage.out ./...

      - name: Check coverage threshold (91%)
        run: |
          coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          if (( $(echo "$coverage < 91" | bc -l) )); then
            echo "Coverage $coverage% is below 91% threshold"
            exit 1
          fi

      - name: Upload coverage to artifact
        uses: actions/upload-artifact@v4
        with:
          name: coverage-report
          path: coverage.out

  # Job 2: Frontend Tests
  frontend-tests:
    name: Frontend Tests (Node.js 18)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '18'
          cache: 'npm'
          cache-dependency-path: fe/package-lock.json

      - name: Install dependencies
        working-directory: fe
        run: npm ci

      - name: Run ESLint
        working-directory: fe
        run: npm run lint

      - name: Run Frontend Tests
        working-directory: fe
        run: npm run test

  # Job 3: Build Verification
  build-check:
    name: Build Verification
    runs-on: ubuntu-latest
    needs: [backend-tests, frontend-tests]
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache: true

      - name: Build gateway CLI
        run: make build-gateway

      - name: Build gatewayd daemon
        run: make build-gatewayd

      - name: Verify binaries exist
        run: |
          test -f bin/gateway || exit 1
          test -f bin/gatewayd || exit 1
          echo "✅ Gateway CLI built successfully"
          echo "✅ Gatewayd daemon built successfully"
```

**Estimated Free Tier Usage**: ~6-10 minutes per run

**Benefits of Simplified Version**:
- ✅ Fast feedback (~6-10 minutes vs ~18 minutes)
- ✅ Catches 90% of common issues
- ✅ Easy to set up (no environment dependencies)
- ✅ Reliable (no flaky integration tests)
- ✅ Low maintenance

---

### Full Version with Integration Tests (OPTIONAL)

Add this job only after the simplified version is stable and working:

```yaml
  # Job 4: Integration Tests (OPTIONAL - Add Later)
  integration-tests:
    name: Integration Tests (All 53 Suites)
    runs-on: ubuntu-latest
    needs: build-check
    if: github.ref == 'refs/heads/main' || github.event_name == 'workflow_dispatch'
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache: true

      - name: Build gatewayd
        run: make build-gatewayd

      - name: Start gatewayd in background
        run: |
          ./bin/gatewayd &
          sleep 5
        env:
          TOKLIGENCE_LOG_LEVEL: debug
          TOKLIGENCE_MARKETPLACE_ENABLED: false
          TOKLIGENCE_AUTH_DISABLED: true

      - name: Run integration tests
        working-directory: tests
        run: ./run_all_tests.sh
        timeout-minutes: 15

      - name: Stop gatewayd
        if: always()
        run: pkill gatewayd || true

      - name: Upload test logs
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: integration-test-logs
          path: logs/
```

**Note**: Integration tests only run on `main` branch or manual trigger to save CI minutes.

**Benefits**:
- Catches bugs before merge
- Ensures 91% coverage threshold
- Validates multi-component integration
- Fast feedback loop (<20 minutes)

---

## 2️⃣ Release Automation Workflow

**File**: `.github/workflows/release.yml`

### Trigger Conditions
- Tag push matching `v*.*.*` (e.g., v0.4.0)
- Manual workflow dispatch

### Features

```yaml
name: Release

on:
  push:
    tags:
      - 'v*.*.*'
  workflow_dispatch:
    inputs:
      version:
        description: 'Release version (e.g., v0.4.0)'
        required: true

jobs:
  # Build for all platforms
  build-multi-platform:
    name: Build ${{ matrix.platform }}
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        include:
          - platform: linux-amd64
            os: ubuntu-latest
            goos: linux
            goarch: amd64
          - platform: linux-arm64
            os: ubuntu-latest
            goos: linux
            goarch: arm64
          - platform: darwin-amd64
            os: macos-latest
            goos: darwin
            goarch: amd64
          - platform: darwin-arm64
            os: macos-latest
            goos: darwin
            goarch: arm64
          - platform: windows-amd64
            os: windows-latest
            goos: windows
            goarch: amd64

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Get version
        id: version
        run: echo "VERSION=${GITHUB_REF#refs/tags/}" >> $GITHUB_OUTPUT

      - name: Build binaries
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: 0
        run: |
          # Build gateway CLI
          go build -ldflags "-s -w -X main.buildVersion=${{ steps.version.outputs.VERSION }}" \
            -o dist/gateway-${{ matrix.platform }}${{ matrix.goos == 'windows' && '.exe' || '' }} \
            ./cmd/gateway

          # Build gatewayd daemon
          go build -ldflags "-s -w -X main.buildVersion=${{ steps.version.outputs.VERSION }}" \
            -o dist/gatewayd-${{ matrix.platform }}${{ matrix.goos == 'windows' && '.exe' || '' }} \
            ./cmd/gatewayd

      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: binaries-${{ matrix.platform }}
          path: dist/*

  # Create GitHub Release
  create-release:
    name: Create GitHub Release
    runs-on: ubuntu-latest
    needs: build-multi-platform
    permissions:
      contents: write

    steps:
      - uses: actions/checkout@v4

      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          path: dist

      - name: Generate checksums
        run: |
          cd dist
          find . -type f -exec sha256sum {} \; > SHA256SUMS.txt

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            dist/**/*
          generate_release_notes: true
          draft: false
          prerelease: false
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Benefits**:
- One-command multi-platform releases
- Automatic GitHub Release creation
- SHA256 checksums for security
- Release notes auto-generation

---

## 3️⃣ Docker Build & Push Workflow

**File**: `.github/workflows/docker.yml`

### Trigger Conditions
- Push to `main` (builds `latest` tag)
- Tag push (builds version tags)
- Pull Requests (build only, no push)

### Features

```yaml
name: Docker

on:
  push:
    branches: [ main ]
    tags: [ 'v*.*.*' ]
  pull_request:
    branches: [ main ]

env:
  REGISTRY_IMAGE_PERSONAL: tokligence/gateway-personal
  REGISTRY_IMAGE_TEAM: tokligence/gateway-team

jobs:
  # Build Personal Edition
  docker-personal:
    name: Build Personal Edition
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to Docker Hub
        if: github.event_name != 'pull_request'
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Login to GitHub Container Registry
        if: github.event_name != 'pull_request'
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Docker meta
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: |
            ${{ env.REGISTRY_IMAGE_PERSONAL }}
            ghcr.io/${{ github.repository }}-personal
          tags: |
            type=ref,event=branch
            type=ref,event=pr
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=raw,value=latest,enable={{is_default_branch}}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: docker/Dockerfile.personal
          platforms: linux/amd64,linux/arm64
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

  # Build Team Edition
  docker-team:
    name: Build Team Edition
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to Docker Hub
        if: github.event_name != 'pull_request'
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Login to GitHub Container Registry
        if: github.event_name != 'pull_request'
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Docker meta
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: |
            ${{ env.REGISTRY_IMAGE_TEAM }}
            ghcr.io/${{ github.repository }}-team
          tags: |
            type=ref,event=branch
            type=ref,event=pr
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=raw,value=latest,enable={{is_default_branch}}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: docker/Dockerfile.team
          platforms: linux/amd64,linux/arm64
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

**Benefits**:
- Multi-arch images (amd64, arm64)
- Pushes to Docker Hub + GitHub Container Registry
- Layer caching (speeds up builds by ~60%)
- Automatic versioning

---

## 4️⃣ Code Quality Workflow (OPTIONAL)

**File**: `.github/workflows/quality.yml`

### Trigger Conditions
- Pull Requests
- Weekly schedule (Monday 00:00 UTC)

### Features

```yaml
name: Code Quality

on:
  pull_request:
  schedule:
    - cron: '0 0 * * 1'  # Every Monday
  workflow_dispatch:

jobs:
  # Go linting
  golangci-lint:
    name: Go Linting
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest
          args: --timeout=5m

  # Security scanning
  security-scan:
    name: Security Scan
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Gosec Security Scanner
        uses: securego/gosec@master
        with:
          args: '-no-fail -fmt sarif -out results.sarif ./...'

      - name: Upload SARIF file
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif

  # Dependency check
  dependency-review:
    name: Dependency Review
    runs-on: ubuntu-latest
    if: github.event_name == 'pull_request'
    steps:
      - uses: actions/checkout@v4

      - name: Dependency Review
        uses: actions/dependency-review-action@v4
```

**Benefits**:
- Catches code smells early
- Security vulnerability detection
- Dependency risk assessment

---

## 5️⃣ CLA (Contributor License Agreement) Workflow

**File**: `.github/workflows/cla.yml`

### Purpose
Automatically ensure all contributors have signed the Contributor License Agreement before their PRs can be merged. This protects the project legally and clarifies IP ownership.

### Implementation Options

#### Option A: CLA Assistant (Recommended)
Most popular GitHub CLA solution, free for open source.

**Setup**:
1. Install GitHub App: https://github.com/apps/cla-assistant
2. Create CLA document (example: `docs/CLA.md`)
3. Add workflow:

```yaml
name: CLA Assistant

on:
  issue_comment:
    types: [created]
  pull_request_target:
    types: [opened, synchronize, reopened]

jobs:
  cla-check:
    runs-on: ubuntu-latest
    steps:
      - name: CLA Assistant
        if: (github.event.comment.body == 'recheck' || github.event.comment.body == 'I have read the CLA Document and I hereby sign the CLA') || github.event_name == 'pull_request_target'
        uses: contributor-assistant/github-action@v2.3.0
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          PERSONAL_ACCESS_TOKEN: ${{ secrets.CLA_BOT_PAT }}
        with:
          path-to-signatures: 'signatures/cla.json'
          path-to-document: 'https://github.com/tokligence/tokligence-gateway/blob/main/docs/CLA.md'
          branch: 'cla-signatures'
          allowlist: 'bot*,dependabot*'
```

**Sample CLA Document** (`docs/CLA.md`):
```markdown
# Contributor License Agreement

By making a contribution to the Tokligence Gateway project, I certify that:

(a) The contribution was created in whole or in part by me and I have the right to submit it under the Apache 2.0 license; or

(b) The contribution is based upon previous work that, to the best of my knowledge, is covered under an appropriate license and I have the right to submit that work with modifications under the Apache 2.0 license; or

(c) The contribution was provided directly to me by some other person who certified (a), (b) or (c) and I have not modified it.

(d) I understand and agree that this project and the contribution are public and that a record of the contribution is maintained indefinitely.

To sign this CLA, comment on the PR: "I have read the CLA Document and I hereby sign the CLA"
```

#### Option B: Simple DCO (Developer Certificate of Origin)
Lighter alternative, requires signed-off commits.

```yaml
name: DCO Check

on:
  pull_request:

jobs:
  dco-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Check DCO
        uses: tim-actions/dco@v1.1
        with:
          require-signoff: true
```

Contributors sign by adding to commit message:
```bash
git commit -s -m "Your commit message"

# Results in:
# Your commit message
#
# Signed-off-by: John Doe <john.doe@example.com>
```

### Recommendation
- **For Community/Enterprise projects**: Use CLA Assistant
- **For smaller projects**: Use DCO (simpler, less friction)
- **For Apache-licensed projects**: DCO is sufficient (matches Apache governance)

---

## 6️⃣ Performance Regression Workflow (OPTIONAL)

**File**: `.github/workflows/performance.yml`

### Trigger Conditions
- Manual workflow dispatch
- Weekly schedule

### Features

```yaml
name: Performance Tests

on:
  workflow_dispatch:
  schedule:
    - cron: '0 2 * * 0'  # Every Sunday at 2 AM UTC

jobs:
  latency-benchmark:
    name: Latency Benchmarks
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Build gatewayd
        run: make build-gatewayd

      - name: Start gatewayd
        run: |
          ./bin/gatewayd &
          sleep 5
        env:
          TOKLIGENCE_LOG_LEVEL: info

      - name: Run latency tests
        working-directory: tests/performance
        run: ./test_latency.sh

      - name: Compare with baseline
        run: |
          # Compare results with baseline (from artifact)
          # Fail if P99 latency increased by >10%
          echo "Performance comparison logic here"

      - name: Upload results
        uses: actions/upload-artifact@v4
        with:
          name: performance-results-${{ github.sha }}
          path: tests/performance/results/
```

**Benefits**:
- Detects performance regressions
- Historical performance tracking
- Baseline comparison

---

## 7️⃣ Dependabot Configuration (RECOMMENDED)

**File**: `.github/dependabot.yml`

### Purpose
Automatically create PRs for dependency updates, keeping your project secure and up-to-date.

### Configuration

```yaml
version: 2
updates:
  # Go dependencies
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
      day: "monday"
      time: "09:00"
      timezone: "Asia/Singapore"
    open-pull-requests-limit: 5
    labels:
      - "dependencies"
      - "go"
    commit-message:
      prefix: "chore(deps)"
    reviewers:
      - "your-team-name"
    assignees:
      - "your-username"

  # npm dependencies (Frontend)
  - package-ecosystem: "npm"
    directory: "/fe"
    schedule:
      interval: "weekly"
      day: "monday"
      time: "09:00"
      timezone: "Asia/Singapore"
    open-pull-requests-limit: 5
    labels:
      - "dependencies"
      - "frontend"
    commit-message:
      prefix: "chore(deps)"
    versioning-strategy: increase
    groups:
      # Group React ecosystem updates
      react:
        patterns:
          - "react*"
          - "@types/react*"
      # Group testing libraries
      testing:
        patterns:
          - "@testing-library/*"
          - "vitest"
          - "@vitest/*"
      # Group build tools
      build-tools:
        patterns:
          - "vite*"
          - "@vitejs/*"
          - "typescript"

  # Docker dependencies
  - package-ecosystem: "docker"
    directory: "/docker"
    schedule:
      interval: "weekly"
      day: "monday"
      time: "09:00"
      timezone: "Asia/Singapore"
    labels:
      - "dependencies"
      - "docker"
    commit-message:
      prefix: "chore(docker)"

  # GitHub Actions
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
      day: "monday"
      time: "09:00"
      timezone: "Asia/Singapore"
    labels:
      - "dependencies"
      - "ci"
    commit-message:
      prefix: "chore(ci)"
```

### Features
- **Weekly updates**: Checks every Monday morning
- **Grouped updates**: React, testing, build tools grouped together
- **Auto-labeling**: PRs tagged with appropriate labels
- **Limited PRs**: Max 5 open PRs per ecosystem (prevents spam)
- **Time zone aware**: Runs during business hours in Asia/Singapore

### Security Alerts
Enable Dependabot security alerts:
1. Go to Settings → Security → Code security and analysis
2. Enable "Dependabot alerts"
3. Enable "Dependabot security updates"

Dependabot will automatically create PRs for critical security vulnerabilities.

---

## 8️⃣ CodeQL Security Scanning (RECOMMENDED)

**File**: `.github/workflows/codeql.yml`

### Purpose
Automated code scanning for security vulnerabilities and coding errors.

### Configuration

```yaml
name: CodeQL

on:
  push:
    branches: [ main, master ]
  pull_request:
    branches: [ main, master ]
  schedule:
    - cron: '0 6 * * 1'  # Every Monday at 6 AM UTC

jobs:
  analyze:
    name: Analyze
    runs-on: ubuntu-latest
    permissions:
      actions: read
      contents: read
      security-events: write

    strategy:
      fail-fast: false
      matrix:
        language: [ 'go', 'javascript' ]

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Initialize CodeQL
        uses: github/codeql-action/init@v3
        with:
          languages: ${{ matrix.language }}
          queries: security-extended,security-and-quality

      - name: Autobuild
        uses: github/codeql-action/autobuild@v3

      - name: Perform CodeQL Analysis
        uses: github/codeql-action/analyze@v3
        with:
          category: "/language:${{ matrix.language }}"
```

### Features
- Scans Go backend and JavaScript frontend
- Runs on every push/PR + weekly schedule
- Uses extended security queries
- Results appear in Security tab

---

## 📋 Implementation Roadmap

### Phase 1: Foundation (Week 1-2) - HIGHEST PRIORITY
- [ ] **Create Core CI Workflow** (`ci.yml`)
  - Backend tests
  - Frontend tests
  - Build verification
  - Integration tests
- [ ] **Configure Dependabot** (`dependabot.yml`)
  - Go modules
  - npm packages
  - Docker images
  - GitHub Actions
- [ ] **Set up Branch Protection Rules**
- [ ] **Add CI badges to README**

**Deliverable**: All tests run automatically on every PR + automated dependency updates

---

### Phase 2: Release Automation (Week 3)
- [ ] **Create Release Workflow** (`release.yml`)
  - Multi-platform builds
  - GitHub Release creation
- [ ] **Create Docker Workflow** (`docker.yml`)
  - Personal Edition builds
  - Team Edition builds
- [ ] **Set up Docker Hub / GHCR credentials**

**Deliverable**: One-command releases for all platforms

---

### Phase 3: Quality & Security (Week 4+)
- [ ] **Add Code Quality Workflow** (`quality.yml`)
  - golangci-lint for Go
  - ESLint for Frontend
  - Gosec security scanner
- [ ] **Add CodeQL Security Scanning** (`codeql.yml`)
  - Go and JavaScript analysis
  - Weekly security scans
- [ ] **Add Performance Testing** (`performance.yml`)
  - Latency benchmarks
  - Performance regression detection
- [ ] **Add CLA/DCO Workflow** (`cla.yml`)
  - Choose: CLA Assistant or DCO
  - Create CLA document if using CLA Assistant
  - Configure contributor signing process

**Deliverable**: Automated quality gates, security scanning, performance tracking, and contributor agreements

---

## 💡 Optimization Strategies

### A. GitHub Actions Free Tier Limits
**Free Organization Plan**:
- 2,000 minutes/month (shared across all repos)
- Max 20 concurrent jobs
- 6 hours max per job

### Optimization Tactics:

1. **Cache Aggressively**
   ```yaml
   # Go modules cache
   - uses: actions/setup-go@v5
     with:
       cache: true  # Saves ~2-3 minutes per run

   # npm cache
   - uses: actions/setup-node@v4
     with:
       cache: 'npm'  # Saves ~1-2 minutes per run
   ```

2. **Run Tests in Parallel**
   - Backend + Frontend in parallel (saves ~5 minutes)
   - Integration tests grouped by category

3. **Conditional Execution**
   ```yaml
   # Only run full integration tests on main branch
   if: github.ref == 'refs/heads/main'

   # Skip Docker builds on PR
   if: github.event_name != 'pull_request'
   ```

4. **Smart Test Selection**
   - PR: Fast tests only (unit + lint) ~8 minutes
   - Merge: Full integration tests ~18 minutes
   - Release: Full suite + performance tests ~30 minutes

**Estimated Monthly Usage**:
- 20 PRs/month × 8 min = 160 minutes
- 20 merges/month × 18 min = 360 minutes
- 4 releases/month × 30 min = 120 minutes
- **Total: ~640 minutes/month** (well within 2,000 limit)

---

## 🛡️ Branch Protection Rules

### Recommended Settings for `main` Branch

```yaml
Settings → Branches → Branch protection rules → Add rule

Branch name pattern: main

Required checks:
  ✅ Require status checks to pass before merging
     - backend-tests
     - frontend-tests
     - build-check

  ✅ Require branches to be up to date before merging

  ✅ Require linear history (optional, for clean history)

  ❌ Require approvals (optional - skip for solo/small teams)

  ✅ Require conversation resolution before merging

  ✅ Do not allow bypassing the above settings
```

---

## 📦 Required Secrets Configuration

### Repository Secrets (Settings → Secrets and variables → Actions)

#### For Docker Publishing:
```
DOCKERHUB_USERNAME=your-dockerhub-username
DOCKERHUB_TOKEN=your-dockerhub-access-token
```

#### For Release Automation (auto-provided by GitHub):
```
GITHUB_TOKEN=<automatically provided>
```

#### Optional - for enhanced features:
```
CODECOV_TOKEN=<if using Codecov>
SLACK_WEBHOOK=<for notifications>
```

---

## 🎖️ README Badges

Add to top of `README.md`:

```markdown
![CI](https://github.com/tokligence/tokligence-gateway/workflows/CI/badge.svg)
![Release](https://github.com/tokligence/tokligence-gateway/workflows/Release/badge.svg)
![Docker](https://github.com/tokligence/tokligence-gateway/workflows/Docker/badge.svg)
![Go Version](https://img.shields.io/github/go-mod/go-version/tokligence/tokligence-gateway)
![License](https://img.shields.io/github/license/tokligence/tokligence-gateway)
![Docker Pulls](https://img.shields.io/docker/pulls/tokligence/gateway-personal)
```

---

## 📊 Success Metrics

### After Full Implementation:

| Metric | Before | After | Target |
|--------|--------|-------|--------|
| Time to detect failures | Manual (~hours) | Automatic (~15 min) | <20 min |
| Release preparation time | Manual (~2 hours) | Automated (~30 min) | <1 hour |
| Test execution frequency | Irregular | Every PR + merge | 100% coverage |
| Code coverage tracking | Manual | Automated | ≥91% maintained |
| Docker image freshness | Manual builds | Auto-updated | <1 day lag |
| Security vulnerability detection | Manual | Weekly scan | 100% coverage |

---

## 🔧 Troubleshooting Common Issues

### Issue 1: Go Module Cache Misses
**Solution**: Use `actions/setup-go@v5` with `cache: true`

### Issue 2: Integration Tests Timeout
**Solution**: Add `timeout-minutes: 15` to job steps

### Issue 3: Docker Build Fails on ARM64
**Solution**: Use `docker/setup-qemu-action@v3` for multi-arch

### Issue 4: Test Flakiness
**Solution**: Add retry logic or increase timeouts in `tests/run_all_tests.sh`

---

## 📚 Additional Resources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [GitHub Actions Best Practices](https://docs.github.com/en/actions/learn-github-actions/best-practices-for-github-actions)
- [Docker Build Push Action](https://github.com/docker/build-push-action)
- [Go Setup Action](https://github.com/actions/setup-go)

---

## ❓ Next Steps

To implement this CI/CD strategy:

1. **Prioritize**: Choose which workflows to implement first
2. **Prepare**: Set up required secrets (Docker Hub tokens, etc.)
3. **Implement**: Create workflow files in `.github/workflows/`
4. **Test**: Run workflows on a test branch first
5. **Deploy**: Enable branch protection rules
6. **Monitor**: Track usage and optimize

**Questions to Answer**:
1. Which registry do you prefer: Docker Hub or GitHub Container Registry (or both)?
2. Do you want coverage reports uploaded to Codecov/Coveralls?
3. Should integration tests run on every PR or only on merge?
4. Do you need Slack/Discord notifications for CI failures?

---

## 📝 Quick Start Checklist

Ready to implement CI/CD? Follow this checklist:

### Pre-Implementation
- [ ] Review all workflow files in this document
- [ ] Decide on priority workflows (Phase 1 minimum)
- [ ] Get necessary approvals from team/organization
- [ ] Identify who will maintain CI/CD pipelines

### Phase 1 Setup (Week 1)
- [ ] Create `.github/workflows/` directory
- [ ] Copy `ci.yml` workflow from this document
- [ ] Copy `dependabot.yml` configuration
- [ ] Test CI workflow on a test branch first
- [ ] Fix any issues (API keys, environment variables, etc.)
- [ ] Merge to main branch

### Phase 1 Configuration (Week 2)
- [ ] Enable branch protection rules for `main`
  - Go to: Settings → Branches → Add rule
  - Require status checks: `backend-tests`, `frontend-tests`, `build-check`
- [ ] Enable Dependabot alerts
  - Go to: Settings → Security → Code security and analysis
- [ ] Add CI badges to README.md
- [ ] Document CI/CD setup for team

### Phase 2 Setup (Week 3)
- [ ] Set up Docker Hub or GHCR credentials
  - Create secrets: `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`
- [ ] Copy `release.yml` workflow
- [ ] Copy `docker.yml` workflow
- [ ] Test release process with a test tag (e.g., v0.0.1-test)
- [ ] Verify Docker images build correctly

### Phase 3 Setup (Week 4+)
- [ ] Copy `quality.yml` workflow
- [ ] Copy `codeql.yml` workflow
- [ ] Decide on CLA vs DCO
- [ ] Copy `cla.yml` or DCO workflow
- [ ] Create CLA document if using CLA Assistant
- [ ] Copy `performance.yml` workflow (optional)

### Post-Implementation
- [ ] Monitor first few runs for issues
- [ ] Adjust timeouts if needed
- [ ] Optimize cache usage
- [ ] Set up notifications (email, Slack, etc.)
- [ ] Train team on new CI/CD workflows
- [ ] Document troubleshooting procedures

### Monthly Maintenance
- [ ] Review Dependabot PRs and merge
- [ ] Check CI/CD usage (GitHub Actions minutes)
- [ ] Review CodeQL security findings
- [ ] Update workflow versions (GitHub Actions)
- [ ] Review and update this document

---

## 🔗 Quick Links

- **GitHub Actions Docs**: https://docs.github.com/en/actions
- **Dependabot Docs**: https://docs.github.com/en/code-security/dependabot
- **CodeQL Docs**: https://codeql.github.com/docs/
- **CLA Assistant**: https://github.com/contributor-assistant/github-action
- **Docker Build Action**: https://github.com/docker/build-push-action
- **This Project's Tests**: `/tests/run_all_tests.sh`
- **This Project's Makefile**: `/Makefile`

---

**Document Maintainer**: DevOps Team
**Review Schedule**: Monthly
**Next Review**: 2025-12-01
**Version**: 1.0
**Last Updated**: 2025-11-13
