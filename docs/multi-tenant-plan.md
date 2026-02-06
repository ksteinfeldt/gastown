# Gas Town Multi-Tenant Architecture Plan

**Version:** 1.0
**Date:** 2026-02-06
**Status:** Planning

## Executive Summary

This document outlines the architectural design and implementation plan for enabling multi-tenant support in Gas Town. The goal is to allow multiple independent towns (tenants) to run simultaneously on a single host, each with isolated resources, agents, and configurations.

**Key Objectives:**
- Enable multiple independent Gas Town instances per host
- Maintain complete isolation between tenants (data, sessions, resources)
- Preserve backward compatibility with single-tenant deployments
- Minimize performance overhead from tenant isolation
- Support seamless tenant switching for administrators

**Expected Benefits:**
- Multi-project isolation for enterprise users
- Hosting multiple client workspaces on shared infrastructure
- Development/staging/production environment separation
- Team-based workspace partitioning

---

## Current Architecture Analysis

### Single-Tenant Model

Gas Town currently operates as a **single-tenant system** with these characteristics:

```
~/gt/                           # Single workspace root
├── mayor/                      # Single Mayor instance
│   ├── town.json              # Town identity
│   └── rigs.json              # Registry of all rigs
├── settings/                   # Global settings
│   ├── config.json            # Agent configuration
│   └── accounts.json          # Claude Code accounts
├── <rig-name>/                # Per-project isolation
│   ├── .beads/                # Rig-specific issue database
│   ├── .repo.git/             # Shared bare repository
│   ├── polecats/              # Worker agents
│   └── crew/                  # User workspaces
```

### Current Isolation Mechanisms

| Layer | Isolation Level | Mechanism |
|-------|----------------|-----------|
| **Rigs** | Strong | Separate directories, databases, git repos |
| **Agents** | Strong | Tmux sessions, process isolation |
| **Beads** | Strong | Per-rig SQLite databases with prefix routing |
| **Git** | Strong | Per-rig bare repos, separate worktrees |
| **Config** | Weak | Town-level settings shared across rigs |
| **Sessions** | Weak | `gt-<rig>-<name>` could collide across towns |

### Current Limitations for Multi-Tenancy

1. **Single workspace root**: `~/gt/` is discovered via `mayor/town.json`
2. **Session naming**: `gt-<rig>-<name>` lacks tenant namespace
3. **Global configuration**: `settings/config.json` shared across all rigs
4. **Environment variables**: `GT_TOWN_ROOT`, `BD_ACTOR` are singleton values
5. **Mayor singleton**: One Mayor per workspace
6. **Beads routing**: Single `routes.jsonl` file
7. **Account management**: Single `accounts.json` file

---

## Multi-Tenant Requirements

### Functional Requirements

#### FR-1: Tenant Isolation
- Each tenant must have completely isolated:
  - File system hierarchy
  - Tmux sessions
  - Database instances
  - Git repositories
  - Configuration files

#### FR-2: Tenant Discovery
- System must support multiple tenants in parallel
- Tenants discoverable via:
  - Environment variable (`GT_TENANT`)
  - Command-line flag (`--tenant`)
  - Interactive selection menu

#### FR-3: Resource Quotas (Future)
- Per-tenant limits on:
  - Number of rigs
  - Number of active agents
  - Disk space
  - CPU/memory (optional)

#### FR-4: Tenant Lifecycle
- Operations required:
  - `gt tenant create <name>` - Create new tenant
  - `gt tenant list` - List all tenants
  - `gt tenant switch <name>` - Switch active tenant
  - `gt tenant delete <name>` - Remove tenant
  - `gt tenant status` - Show current tenant

#### FR-5: Backward Compatibility
- Existing single-tenant deployments must continue working
- Default tenant (`default` or detected from current structure)
- Automatic migration path for existing installations

### Non-Functional Requirements

#### NFR-1: Performance
- Tenant isolation must not introduce >5% overhead
- Tenant switching should complete in <1 second

#### NFR-2: Security
- No cross-tenant data leakage
- Process isolation via filesystem permissions
- Beads database isolation via separate SQLite files

#### NFR-3: Usability
- Tenant selection intuitive and discoverable
- Clear error messages when tenant not found
- Visual indicators of current tenant in CLI

#### NFR-4: Scalability
- Support 10+ tenants per host
- Graceful degradation with resource limits

---

## Proposed Architecture

### Multi-Tenant Hierarchy

```
~/gt/                                    # Multi-tenant root
├── .gt-config                          # Global GT configuration
│   ├── tenants.json                   # Tenant registry
│   ├── active-tenant                  # Current tenant symlink
│   └── quotas.json                    # Resource limits per tenant
├── tenants/                            # Tenant namespace
│   ├── default/                       # Default tenant (backward compat)
│   │   ├── mayor/                     # Tenant-scoped Mayor
│   │   │   ├── town.json             # Tenant identity
│   │   │   └── rigs.json             # Tenant's rigs
│   │   ├── settings/                  # Tenant settings
│   │   │   ├── config.json           # Tenant agent config
│   │   │   └── accounts.json         # Tenant Claude accounts
│   │   ├── <rig-name>/               # Rig (unchanged)
│   │   │   ├── .beads/               # Rig beads
│   │   │   ├── .repo.git/            # Rig repo
│   │   │   ├── polecats/             # Rig polecats
│   │   │   └── crew/                 # Rig crew
│   │   └── .beads/                    # Tenant-level beads routing
│   │       └── routes.jsonl          # Tenant prefix routes
│   ├── acme-corp/                     # Additional tenant
│   │   ├── mayor/
│   │   ├── settings/
│   │   └── <rig-name>/
│   └── staging/                       # Another tenant
│       ├── mayor/
│       ├── settings/
│       └── <rig-name>/
```

### Tenant Identification Strategy

#### Option A: Environment-Based (Recommended)
```bash
export GT_TENANT=acme-corp
gt mayor attach  # Uses acme-corp tenant
```

**Pros:**
- Non-invasive to existing commands
- Aligns with existing `GT_TOWN_ROOT` pattern
- Shell-level context preservation

**Cons:**
- Requires env var management
- Error-prone if forgotten

#### Option B: Command-Flag Based
```bash
gt --tenant=acme-corp mayor attach
gt -t acme-corp agents
```

**Pros:**
- Explicit tenant per command
- No environment pollution

**Cons:**
- Verbose for repeated commands
- Requires all commands support flag

#### Option C: Active Tenant (Hybrid - Recommended)
```bash
gt tenant switch acme-corp      # Sets active tenant in config
gt mayor attach                 # Uses active tenant
GT_TENANT=staging gt agents     # Override with env var
```

**Pros:**
- Best of both approaches
- Explicit switching with implicit usage
- Override capability via env var

**Cons:**
- Requires persistent state management

### Namespace Collision Avoidance

#### Tmux Session Naming
```
Current: gt-<rig>-<name>
Proposed: gt-<tenant>-<rig>-<name>

Examples:
- gt-default-gastown-Toast
- gt-acme-corp-api-Worker1
- gt-staging-frontend-crew-alice
```

#### Agent Identity Format
```
Current: <rig>/<role>/<name>
Proposed: <tenant>-<rig>/<role>/<name>

Examples:
- default-gastown/polecats/Toast
- acme-corp-api/crew/alice
- staging-web/witness
```

#### Beads Prefix Format
```
Current: <rig>-<id>  (e.g., gt-abc12)
Proposed: <tenant>-<rig>-<id>  (e.g., default-gt-abc12)

Examples:
- default-gt-abc12
- acme-api-x7k2m
- staging-web-p9n4q
```

---

## Implementation Phases

### Phase 1: Foundation (Weeks 1-2)

**Goal:** Establish tenant namespace and detection infrastructure

**Tasks:**
1. Create tenant registry system
   - `internal/tenant/registry.go` - Tenant CRUD operations
   - `~/.gt-config/tenants.json` - Persistent tenant list
   - Tenant struct with metadata (name, created, owner, quotas)

2. Implement tenant discovery
   - `internal/tenant/discovery.go` - Active tenant resolution
   - Priority: CLI flag > `GT_TENANT` env var > active tenant file > default
   - Validate tenant exists before operations

3. Extend CLI with tenant commands
   - `cmd/tenant.go` - Tenant management subcommands
   - `gt tenant create <name>` - Initialize new tenant
   - `gt tenant list` - Display all tenants
   - `gt tenant switch <name>` - Set active tenant
   - `gt tenant status` - Show current tenant

4. Update workspace discovery
   - Modify `internal/workspace/workspace.go`
   - `FindWithTenant(tenant string)` - Locate tenant root
   - Backward compatibility: Detect legacy `~/gt/mayor/town.json`

**Deliverables:**
- Tenant registry implementation
- Basic tenant CLI commands
- Tenant-aware workspace discovery

**Testing:**
- Unit tests for tenant registry
- Integration tests for tenant switching
- Backward compatibility validation

---

### Phase 2: Path Resolution (Weeks 3-4)

**Goal:** Update all path construction to be tenant-aware

**Tasks:**
1. Audit path construction across codebase
   - Search for `filepath.Join(townRoot, ...)` patterns
   - Identify hardcoded path assumptions
   - Document all path construction sites

2. Create tenant-aware path builders
   - `internal/tenant/paths.go` - Centralized path resolution
   - Functions:
     - `GetTenantRoot(tenant string) string`
     - `GetMayorDir(tenant string) string`
     - `GetSettingsDir(tenant string) string`
     - `GetRigDir(tenant, rig string) string`

3. Update configuration loading
   - `internal/config/loader.go` - Tenant-scoped config paths
   - Hierarchical resolution: Global → Tenant → Rig → Role
   - Account management: Per-tenant `accounts.json`

4. Update database paths
   - Beads: Tenant-scoped routing files
   - Convoy: Tenant-scoped convoy database
   - Session state: Tenant-prefixed state files

**Deliverables:**
- Centralized path resolution library
- Updated config loaders
- Tenant-scoped database paths

**Testing:**
- Path resolution unit tests
- Config loading integration tests
- Database isolation verification

---

### Phase 3: Session Isolation (Weeks 5-6)

**Goal:** Prevent cross-tenant session collisions

**Tasks:**
1. Update tmux session naming
   - `internal/tmux/session.go` - Tenant-prefixed session names
   - Format: `gt-<tenant>-<rig>-<agent>`
   - Update session discovery logic

2. Update agent identity format
   - `internal/agent/identity.go` - Tenant-aware identities
   - Format: `<tenant>-<rig>/<role>/<name>`
   - Update beads assignee field format

3. Update environment variable injection
   - `internal/tmux/environment.go` - Set `GT_TENANT` per session
   - Ensure `BD_ACTOR` includes tenant prefix
   - Preserve `GT_TOWN_ROOT` for tenant-specific root

4. Update session lifecycle
   - `internal/session/manager.go` - Tenant-aware session operations
   - Session listing filtered by tenant
   - Cross-tenant session prevention

**Deliverables:**
- Tenant-prefixed tmux sessions
- Updated agent identity format
- Tenant-aware environment variables

**Testing:**
- Session naming collision tests
- Cross-tenant session prevention tests
- Environment variable propagation tests

---

### Phase 4: Beads Integration (Weeks 7-8)

**Goal:** Enable tenant-scoped issue tracking

**Tasks:**
1. Update beads prefix format
   - `internal/beads/routing.go` - Tenant-aware prefix generation
   - Format: `<tenant>-<rig>-<id>`
   - Backward compatibility for unprefixed issues

2. Update routes.jsonl structure
   - Per-tenant routing files: `~/gt/tenants/<tenant>/.beads/routes.jsonl`
   - Route resolution includes tenant context
   - Migration tool for existing routes

3. Update beads directory resolution
   - `internal/beads/resolve.go` - Tenant-aware beads dir lookup
   - Redirect files support tenant-scoped paths
   - Validate tenant boundary enforcement

4. Update issue creation/query
   - `internal/beads/client.go` - Pass tenant context to bd CLI
   - Filter queries by tenant prefix
   - Cross-tenant issue references (optional: `@tenant:issue-id`)

**Deliverables:**
- Tenant-prefixed beads format
- Per-tenant routing files
- Updated beads CLI integration

**Testing:**
- Beads prefix generation tests
- Route resolution tests
- Cross-tenant isolation verification

---

### Phase 5: Migration & Compatibility (Weeks 9-10)

**Goal:** Ensure smooth migration from single-tenant to multi-tenant

**Tasks:**
1. Implement migration tool
   - `cmd/migrate.go` - Migrate existing installation
   - Detect legacy structure (`~/gt/mayor/town.json`)
   - Migrate to `~/gt/tenants/default/`
   - Preserve all git history and state

2. Create backward compatibility layer
   - `internal/compat/legacy.go` - Support legacy paths
   - Fallback to `~/gt/` if tenant structure absent
   - Deprecation warnings for legacy structure

3. Update installation process
   - `internal/boot/install.go` - Create multi-tenant structure by default
   - `gt install ~/gt --tenant=default` - Explicit tenant creation
   - Interactive tenant name prompt

4. Document migration process
   - Create `docs/migration-guide.md`
   - Step-by-step migration instructions
   - Rollback procedures

**Deliverables:**
- Migration tool for existing installations
- Backward compatibility layer
- Migration documentation

**Testing:**
- Migration from legacy structure tests
- Backward compatibility validation
- Rollback procedure verification

---

### Phase 6: UI/UX Enhancements (Weeks 11-12)

**Goal:** Improve tenant visibility and usability

**Tasks:**
1. Add tenant indicators to CLI
   - `internal/ui/prompt.go` - Show current tenant in prompts
   - Format: `[acme-corp] gt> `
   - Color coding per tenant (via theme system)

2. Update dashboard for multi-tenancy
   - `internal/web/dashboard.go` - Tenant selector dropdown
   - Per-tenant views for agents, convoys, rigs
   - Cross-tenant summary view (optional)

3. Add tenant context to logs
   - `internal/townlog/logger.go` - Include tenant in log entries
   - Format: `[acme-corp] [gastown] [Toast] <message>`
   - Tenant-scoped log files

4. Improve error messages
   - Tenant-specific error messages
   - Suggest `gt tenant switch` when tenant mismatch
   - Clear indication of tenant boundaries

**Deliverables:**
- Tenant indicators in CLI/TUI
- Multi-tenant dashboard
- Enhanced logging and error messages

**Testing:**
- UI/UX usability testing
- Dashboard tenant switching tests
- Log isolation verification

---

### Phase 7: Resource Management (Weeks 13-14)

**Goal:** Implement tenant resource quotas and monitoring

**Tasks:**
1. Define quota system
   - `internal/tenant/quotas.go` - Quota definitions and enforcement
   - Quotas:
     - Max rigs per tenant
     - Max active agents per tenant
     - Max disk usage per tenant (soft limit)

2. Implement quota enforcement
   - Check quotas before resource allocation
   - Soft limits with warnings
   - Hard limits with errors

3. Add resource monitoring
   - `gt tenant usage` - Show tenant resource usage
   - Disk usage calculation
   - Active agent count
   - Rig count

4. Implement cleanup utilities
   - `gt tenant clean` - Archive inactive tenants
   - `gt tenant prune` - Remove old artifacts
   - Configurable retention policies

**Deliverables:**
- Quota system implementation
- Resource monitoring tools
- Cleanup utilities

**Testing:**
- Quota enforcement tests
- Resource calculation accuracy tests
- Cleanup utility tests

---

## Technical Design Details

### Tenant Registry Schema

```go
// internal/tenant/types.go
type Tenant struct {
    Name        string            `json:"name"`
    DisplayName string            `json:"display_name,omitempty"`
    Owner       string            `json:"owner,omitempty"`
    Created     time.Time         `json:"created"`
    Updated     time.Time         `json:"updated"`
    Active      bool              `json:"active"`
    Quotas      TenantQuotas      `json:"quotas,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
}

type TenantQuotas struct {
    MaxRigs        int   `json:"max_rigs,omitempty"`        // 0 = unlimited
    MaxAgents      int   `json:"max_agents,omitempty"`      // 0 = unlimited
    MaxDiskMB      int64 `json:"max_disk_mb,omitempty"`     // 0 = unlimited
}

type TenantRegistry struct {
    Tenants       []Tenant  `json:"tenants"`
    ActiveTenant  string    `json:"active_tenant"`
    Version       int       `json:"version"`
}
```

### Tenant Context Propagation

```go
// internal/tenant/context.go
type Context struct {
    Name      string
    Root      string
    Registry  *TenantRegistry
}

func GetContext() (*Context, error) {
    // Priority: CLI flag > GT_TENANT env > active tenant > default
    tenant := flagTenant
    if tenant == "" {
        tenant = os.Getenv("GT_TENANT")
    }
    if tenant == "" {
        tenant = registry.ActiveTenant
    }
    if tenant == "" {
        tenant = "default"
    }

    return &Context{
        Name: tenant,
        Root: filepath.Join(gtRoot, "tenants", tenant),
    }, nil
}
```

### Path Resolution Functions

```go
// internal/tenant/paths.go
func (ctx *Context) GetMayorDir() string {
    return filepath.Join(ctx.Root, "mayor")
}

func (ctx *Context) GetSettingsDir() string {
    return filepath.Join(ctx.Root, "settings")
}

func (ctx *Context) GetRigDir(rigName string) string {
    return filepath.Join(ctx.Root, rigName)
}

func (ctx *Context) GetBeadsRoutingFile() string {
    return filepath.Join(ctx.Root, ".beads", "routes.jsonl")
}
```

### Session Naming Convention

```go
// internal/tmux/naming.go
func GetSessionName(tenant, rig, agent string) string {
    if tenant == "" {
        tenant = "default"
    }
    return fmt.Sprintf("gt-%s-%s-%s", tenant, rig, agent)
}

func GetMayorSessionName(tenant string) string {
    if tenant == "" {
        tenant = "default"
    }
    return fmt.Sprintf("gt-%s-mayor", tenant)
}
```

### Beads Prefix Generation

```go
// internal/beads/prefix.go
func GenerateIssueID(tenant, rig string) string {
    id := generateRandomID(5) // e.g., "abc12"
    if tenant == "default" {
        // Backward compatibility: default tenant uses short format
        return fmt.Sprintf("%s-%s", rig, id)
    }
    return fmt.Sprintf("%s-%s-%s", tenant, rig, id)
}

func ParseIssueID(issueID string) (tenant, rig, id string, err error) {
    parts := strings.Split(issueID, "-")
    if len(parts) == 2 {
        // Legacy format: <rig>-<id>
        return "default", parts[0], parts[1], nil
    }
    if len(parts) == 3 {
        // New format: <tenant>-<rig>-<id>
        return parts[0], parts[1], parts[2], nil
    }
    return "", "", "", fmt.Errorf("invalid issue ID format: %s", issueID)
}
```

---

## Migration Strategy

### Automatic Detection & Migration

```bash
# Detect legacy installation
$ gt tenant migrate

Detected legacy Gas Town installation at ~/gt/
This installation will be migrated to multi-tenant structure.

Migration plan:
1. Create tenant namespace: ~/gt/tenants/default/
2. Move mayor/ → tenants/default/mayor/
3. Move settings/ → tenants/default/settings/
4. Move <rigs>/ → tenants/default/<rigs>/
5. Update beads routing files
6. Update session names (will require agent restarts)
7. Create tenant registry

⚠️  Warning: This operation is safe but will require restarting all agents.

Proceed with migration? [y/N]: y

✓ Created tenant namespace
✓ Migrated mayor directory
✓ Migrated settings
✓ Migrated rigs (3 found)
✓ Updated beads routing
✓ Created tenant registry

Migration complete! Your installation is now multi-tenant ready.
Active tenant set to: default

Next steps:
- Restart all agents: gt mayor detach && gt mayor attach
- Create additional tenants: gt tenant create <name>
- Switch between tenants: gt tenant switch <name>
```

### Manual Migration Process

For users preferring manual control:

```bash
# 1. Backup existing installation
cp -r ~/gt ~/gt.backup

# 2. Create tenant structure
mkdir -p ~/gt/tenants/default
mkdir -p ~/gt/.gt-config

# 3. Move existing directories
mv ~/gt/mayor ~/gt/tenants/default/
mv ~/gt/settings ~/gt/tenants/default/
mv ~/gt/<rig-name> ~/gt/tenants/default/

# 4. Initialize tenant registry
cat > ~/gt/.gt-config/tenants.json <<EOF
{
  "tenants": [
    {
      "name": "default",
      "created": "$(date -Iseconds)",
      "active": true
    }
  ],
  "active_tenant": "default",
  "version": 1
}
EOF

# 5. Update beads routing (if exists)
if [ -f ~/gt/.beads/routes.jsonl ]; then
  mkdir -p ~/gt/tenants/default/.beads
  mv ~/gt/.beads/routes.jsonl ~/gt/tenants/default/.beads/
fi

# 6. Verify migration
gt tenant list
gt tenant status
```

### Rollback Procedure

If migration fails or causes issues:

```bash
# 1. Stop all agents
gt agents --stop-all

# 2. Restore from backup
rm -rf ~/gt
mv ~/gt.backup ~/gt

# 3. Restart agents
gt mayor attach
```

---

## Security Considerations

### Tenant Isolation

1. **Filesystem Isolation**
   - Each tenant directory owned by user (0755 permissions)
   - Prevent symlink attacks across tenant boundaries
   - Validate all paths stay within tenant root

2. **Process Isolation**
   - Tmux sessions namespace-isolated per tenant
   - Environment variables scoped to tenant
   - No shared process state between tenants

3. **Database Isolation**
   - Separate SQLite databases per tenant
   - No cross-tenant database connections
   - Beads prefix validation prevents cross-tenant issue access

4. **Git Repository Isolation**
   - Separate bare repos per tenant-rig combination
   - No shared worktrees across tenants
   - Separate credential storage per tenant (accounts.json)

### Access Control

Current model:
- Single-user system (filesystem permissions only)
- All tenants accessible to owner

Future enhancements:
- Multi-user support with tenant-level ACLs
- Role-based access control (RBAC) per tenant
- Audit logging for tenant operations

### Data Validation

- Validate tenant names (alphanumeric, no special chars except dash/underscore)
- Validate paths don't escape tenant boundaries
- Validate beads prefixes match tenant context
- Sanitize all user input for tenant names

---

## Testing Strategy

### Unit Tests

**Components:**
- `internal/tenant/registry_test.go` - Tenant CRUD operations
- `internal/tenant/discovery_test.go` - Tenant resolution logic
- `internal/tenant/paths_test.go` - Path construction
- `internal/beads/prefix_test.go` - Prefix generation/parsing

**Coverage Target:** >80% for tenant-related code

### Integration Tests

**Scenarios:**
1. Create tenant → Add rig → Spawn agent → Verify isolation
2. Switch tenant → Verify context change → Verify session isolation
3. Migrate legacy → Verify data integrity → Verify backward compat
4. Create cross-tenant issue refs → Verify rejection
5. Quota enforcement → Create rigs until limit → Verify rejection

### Load Tests

**Scenarios:**
- 10 tenants with 5 rigs each (50 rigs total)
- 100 active agents across tenants
- Measure tenant switching latency
- Measure resource overhead per tenant

### Regression Tests

- Ensure single-tenant deployments continue working
- Verify existing rig/agent operations unaffected
- Test beads routing with legacy format
- Validate tmux session naming backward compat

---

## Rollout Plan

### Alpha Release (Internal Testing)

**Duration:** 2 weeks
**Audience:** Gas Town core team

**Goals:**
- Validate core tenant functionality
- Identify edge cases and bugs
- Refine CLI UX
- Performance benchmarking

**Acceptance Criteria:**
- All unit tests passing
- Integration tests covering key workflows
- Migration tool tested on real installations
- Documentation complete

### Beta Release (Early Adopters)

**Duration:** 4 weeks
**Audience:** Selected power users

**Goals:**
- Real-world usage validation
- Identify usability issues
- Gather feedback on UX
- Stress testing with diverse workloads

**Acceptance Criteria:**
- No critical bugs in beta period
- Positive feedback from >80% of beta testers
- Performance meets SLA (no >5% overhead)
- Migration success rate >95%

### General Availability (GA)

**Prerequisites:**
- All beta feedback addressed
- Comprehensive documentation published
- Migration guide validated
- Rollback procedures tested

**Communication:**
- Release notes highlighting multi-tenant features
- Migration guide published
- Video tutorial for tenant management
- FAQ document

---

## Open Questions & Decisions

### Q1: Should default tenant be implicit or explicit?

**Options:**
- **A:** Always require tenant specification (explicit)
- **B:** Default to "default" tenant if unspecified (implicit)
- **C:** Auto-detect single tenant and use it (smart implicit)

**Recommendation:** Option C for best UX

**Rationale:**
- Single-tenant users don't need tenant awareness
- Multi-tenant users benefit from explicit tenant selection
- Avoids confusion for new users

---

### Q2: Should tenants support nested hierarchies?

**Options:**
- **A:** Flat tenant namespace (no nesting)
- **B:** Support tenant hierarchies (e.g., `acme/dev`, `acme/prod`)

**Recommendation:** Option A for v1, Option B for future enhancement

**Rationale:**
- Flat namespace simpler to implement and reason about
- Hierarchies add complexity for uncertain benefit
- Can be added in future without breaking changes

---

### Q3: How to handle cross-tenant collaboration?

**Options:**
- **A:** No cross-tenant features (strict isolation)
- **B:** Support cross-tenant issue references (`@tenant:issue-id`)
- **C:** Support cross-tenant beads sharing (shared database)

**Recommendation:** Option A for v1, Option B as future enhancement

**Rationale:**
- Strict isolation easier to implement securely
- Cross-tenant references can be added later
- Most use cases don't require cross-tenant features

---

### Q4: Should tenants have separate beads servers?

**Options:**
- **A:** Shared beads server with logical isolation
- **B:** Separate beads server process per tenant
- **C:** Server pools (one server per N tenants)

**Recommendation:** Option A for v1

**Rationale:**
- Beads already supports multiple databases via routing
- Separate processes add complexity and overhead
- Routing provides sufficient isolation

---

### Q5: How to handle tenant deletion?

**Options:**
- **A:** Soft delete (mark inactive, keep data)
- **B:** Hard delete (immediate removal)
- **C:** Archive (move to archive directory)

**Recommendation:** Option C

**Rationale:**
- Safety net against accidental deletion
- Allows recovery if needed
- Can add expiration policy later

---

## Success Metrics

### Technical Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Migration success rate | >95% | Automated migration tests |
| Performance overhead | <5% | Benchmark suite |
| Tenant switching latency | <1s | CLI latency measurement |
| Cross-tenant isolation | 100% | Security audit |
| Test coverage | >80% | Code coverage tools |

### User Experience Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| User satisfaction | >4/5 stars | Beta feedback surveys |
| Feature adoption | >50% of users | Usage analytics |
| Migration issues | <5% of users | Support tickets |
| Documentation clarity | >4/5 stars | Documentation feedback |

---

## Risks & Mitigations

### Risk 1: Breaking Existing Installations

**Likelihood:** Medium
**Impact:** High

**Mitigation:**
- Comprehensive migration tool with rollback capability
- Extensive backward compatibility testing
- Beta program to catch issues early
- Clear migration documentation
- Support channel for migration assistance

### Risk 2: Performance Degradation

**Likelihood:** Low
**Impact:** Medium

**Mitigation:**
- Performance benchmarking in alpha/beta
- Lazy loading of tenant contexts
- Efficient path resolution caching
- Resource monitoring and alerts

### Risk 3: Incomplete Isolation

**Likelihood:** Low
**Impact:** High

**Mitigation:**
- Security audit of all tenant boundaries
- Automated isolation testing
- Code review focused on cross-tenant access
- Penetration testing in beta

### Risk 4: User Confusion

**Likelihood:** Medium
**Impact:** Medium

**Mitigation:**
- Smart defaults (auto-detect single tenant)
- Clear CLI prompts showing current tenant
- Comprehensive documentation
- Interactive tutorials
- Helpful error messages

---

## Future Enhancements

### Short-term (3-6 months)

1. **Tenant templates**: Pre-configured tenant blueprints
2. **Cross-tenant references**: `@tenant:issue-id` format for linking
3. **Tenant themes**: Visual differentiation per tenant
4. **Resource dashboards**: Real-time tenant resource monitoring

### Medium-term (6-12 months)

1. **Multi-user support**: Tenant-level user management and ACLs
2. **Cloud sync**: Sync tenant state to cloud storage
3. **Tenant hierarchies**: Support `org/team/project` nesting
4. **Tenant backups**: Automated backup/restore per tenant
5. **Tenant APIs**: HTTP API for tenant management

### Long-term (12+ months)

1. **Distributed tenants**: Multi-host tenant coordination
2. **Tenant federation**: Cross-instance tenant collaboration
3. **Enterprise features**: SSO, LDAP integration, audit logging
4. **Tenant marketplace**: Shareable tenant templates and workflows

---

## Appendix A: File Structure Examples

### Legacy (Single-Tenant)
```
~/gt/
├── mayor/
│   ├── town.json
│   └── rigs.json
├── settings/
│   ├── config.json
│   └── accounts.json
├── gastown/
│   ├── .beads/
│   ├── .repo.git/
│   ├── polecats/
│   └── crew/
└── pixelforge/
    ├── .beads/
    └── crew/
```

### Multi-Tenant (Migrated)
```
~/gt/
├── .gt-config/
│   ├── tenants.json
│   └── active-tenant
├── tenants/
│   ├── default/
│   │   ├── mayor/
│   │   │   ├── town.json
│   │   │   └── rigs.json
│   │   ├── settings/
│   │   │   ├── config.json
│   │   │   └── accounts.json
│   │   ├── gastown/
│   │   │   ├── .beads/
│   │   │   ├── .repo.git/
│   │   │   ├── polecats/
│   │   │   └── crew/
│   │   └── pixelforge/
│   ├── acme-corp/
│   │   ├── mayor/
│   │   ├── settings/
│   │   └── api-server/
│   └── staging/
│       ├── mayor/
│       ├── settings/
│       └── frontend/
```

---

## Appendix B: CLI Examples

### Tenant Management
```bash
# List all tenants
$ gt tenant list
Tenants:
  * default (active) - Created 2026-01-15
  acme-corp - Created 2026-02-01
  staging - Created 2026-02-03

# Create new tenant
$ gt tenant create acme-corp --owner="Alice Smith"
Created tenant: acme-corp

# Switch active tenant
$ gt tenant switch acme-corp
Switched to tenant: acme-corp

# Show current tenant
$ gt tenant status
Current tenant: acme-corp
Rigs: 3
Active agents: 7
Disk usage: 2.3 GB

# Delete tenant (with confirmation)
$ gt tenant delete staging
⚠️  This will archive tenant 'staging' and all its data.
Type 'staging' to confirm: staging
Archived tenant: staging → ~/gt/.archives/staging-20260206

# Show tenant resource usage
$ gt tenant usage acme-corp
Tenant: acme-corp
Rigs: 3 / 10 (quota)
Active agents: 7 / 20 (quota)
Disk usage: 2.3 GB / 10 GB (quota)
```

### Tenant Context in Operations
```bash
# Explicit tenant via flag
$ gt --tenant=acme-corp mayor attach

# Explicit tenant via env var
$ GT_TENANT=acme-corp gt agents

# Implicit tenant (uses active tenant)
$ gt tenant switch acme-corp
$ gt mayor attach  # Uses acme-corp

# Override active tenant with env var
$ gt tenant switch default
$ GT_TENANT=acme-corp gt agents  # Uses acme-corp, not default
```

### Migration
```bash
# Automatic migration
$ gt tenant migrate
Detected legacy installation. Migrating to multi-tenant structure...
✓ Migration complete

# Migration with custom tenant name
$ gt tenant migrate --name=production
Migrating to tenant: production
✓ Migration complete

# Dry-run migration (preview only)
$ gt tenant migrate --dry-run
Migration plan:
1. Create ~/gt/tenants/default/
2. Move mayor/ → tenants/default/mayor/
3. Move settings/ → tenants/default/settings/
4. Move 3 rigs → tenants/default/
```

---

## Appendix C: Configuration Examples

### Tenant Registry (`~/.gt-config/tenants.json`)
```json
{
  "tenants": [
    {
      "name": "default",
      "display_name": "Default Workspace",
      "owner": "alice@example.com",
      "created": "2026-01-15T10:00:00Z",
      "updated": "2026-02-06T14:30:00Z",
      "active": true,
      "quotas": {
        "max_rigs": 10,
        "max_agents": 50,
        "max_disk_mb": 10240
      },
      "metadata": {
        "team": "platform",
        "environment": "development"
      }
    },
    {
      "name": "acme-corp",
      "display_name": "Acme Corp Projects",
      "owner": "bob@acme.com",
      "created": "2026-02-01T09:00:00Z",
      "updated": "2026-02-06T14:30:00Z",
      "active": true,
      "quotas": {
        "max_rigs": 5,
        "max_agents": 20,
        "max_disk_mb": 5120
      }
    }
  ],
  "active_tenant": "acme-corp",
  "version": 1
}
```

### Per-Tenant Beads Routing (`~/gt/tenants/acme-corp/.beads/routes.jsonl`)
```jsonl
{"prefix":"acme-api","rig":"api-server"}
{"prefix":"acme-web","rig":"frontend"}
{"prefix":"acme-db","rig":"database"}
```

---

## Appendix D: References

### Internal Documentation
- [Gas Town Architecture](overview.md)
- [Beads Integration](beads-native-messaging.md)
- [Formula Resolution](formula-resolution.md)
- [Glossary](glossary.md)

### External References
- [SQLite Multi-Database](https://www.sqlite.org/lang_attach.html)
- [Git Worktrees](https://git-scm.com/docs/git-worktree)
- [Tmux Session Management](https://github.com/tmux/tmux/wiki)

### Related Issues
- TBD: Create tracking issues once plan approved

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-02-06 | Planning Agent | Initial comprehensive plan |

---

## Approval & Sign-off

**Plan Status:** Draft - Awaiting Review

**Reviewers:**
- [ ] Engineering Lead
- [ ] Product Manager
- [ ] Security Review
- [ ] Documentation Review

**Next Steps:**
1. Review and approve plan
2. Create tracking issues for each phase
3. Assign engineering resources
4. Begin Phase 1 implementation

---

*End of Multi-Tenant Architecture Plan*
