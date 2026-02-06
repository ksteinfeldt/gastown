# Gas Town Multi-Overseer Architecture Plan

**Version:** 1.0
**Date:** 2026-02-06
**Status:** Planning

## Executive Summary

This document outlines the architectural design for enabling **multiple human overseers** (users) to collaborate within a **single Gas Town instance**. This differs from multi-tenant architecture (which provides complete isolation between separate town instances). The multi-overseer model creates a collaborative workspace where multiple users can work together in one shared environment, each owning their own rigs but with visibility into other users' work.

**Key Objectives:**
- Enable multiple human users in a single Gas Town instance
- Each user owns their own rigs (no rig sharing)
- Users can see each other and coordinate work
- Clear ownership and attribution per user
- Simplified collaboration model (not enterprise multi-tenancy)

**Expected Benefits:**
- Team collaboration in shared development environment
- Clear attribution of work to specific users
- Coordinated agent dispatch across team members
- Shared visibility for project coordination
- Simpler than full multi-tenant isolation

**NOT in Scope:**
- Multi-tenant isolation (separate town instances)
- Rig sharing between users
- Enterprise features (LDAP, SSO, RBAC)
- Complete data isolation between users

---

## Current Architecture Analysis

### Single-Overseer Model

Gas Town currently operates as a **single-overseer system**:

```
~/gt/                           # Single workspace root
├── .beads/                     # Town-level beads (hq-* prefix, mail)
├── mayor/                      # Mayor config
│   ├── town.json              # Town identity
│   ├── overseer.json          # Single overseer identity
│   └── rigs.json              # Registry of all rigs
├── settings/                   # Global settings
│   ├── config.json            # Agent configuration
│   └── accounts.json          # Claude Code accounts
├── deacon/                     # Background supervisor daemon
│   └── dogs/                   # Deacon helpers
└── <rig-name>/                # Per-project rig
    ├── config.json             # Rig identity
    ├── .beads/                 # Rig-specific issue database
    ├── .repo.git/              # Shared bare repository
    ├── mayor/rig/              # Mayor's canonical clone
    ├── refinery/rig/           # Refinery worktree on main
    ├── witness/                # Witness (no clone)
    ├── crew/                   # User workspaces
    │   ├── joe/                # Crew member
    │   └── beads-wolf/         # Cross-rig worktree
    └── polecats/               # Ephemeral worker worktrees
        └── Toast/              # Individual polecat
```

### Current Identity Model

**Overseer Identity:**
- Single `mayor/overseer.json` file
- Detected from git config, GitHub CLI, or environment
- Format: `{name, email, username, source}`

**Agent Identity:**
- Format: `<rig>/<role>/<name>`
- Examples: `gastown/polecats/Toast`, `beads/crew/joe`
- No user context in identity

**Work Attribution:**
- Git commits: `Author: <rig>/<role>/<name> <overseer@email>`
- Beads issues: `created_by: <rig>/<role>/<name>`
- Events: `actor: <rig>/<role>/<name>`

### Current Limitations for Multi-Overseer

1. **Single overseer identity**: Only one `mayor/overseer.json`
2. **No user-scoped rig ownership**: All rigs owned by single overseer
3. **No user context in agent identity**: Can't distinguish which user's agent
4. **Global settings**: Single `settings/` shared by all (would be fine for shared town)
5. **Mail routing**: No user-to-user routing concept
6. **Mayor singleton**: One Mayor per workspace (fine for shared model)
7. **Beads visibility**: No user-scoped filtering
8. **Session management**: No user context in tmux sessions

---

## Multi-Overseer Requirements

### Functional Requirements

#### FR-1: User Identity and Authentication

Each user must have:
- **Persistent identity**: username, display name, email
- **Authentication**: initial auth on first use (can be local, no enterprise auth needed)
- **Identity propagation**: user context flows to all operations

#### FR-2: User-Scoped Rig Ownership

- Each rig belongs to exactly one user
- Owner is set at rig creation time
- Owner can assign work to their rigs' agents
- Rigs cannot be transferred between users (v1 simplification)

#### FR-3: Updated Agent Identity

Agent identity must include user context:
- Format: `<user>/<rig>/<role>/<name>` or similar
- Enables attribution: "which user's agent did this work?"
- Preserved in commits, beads, events

#### FR-4: Cross-User Visibility

Users should be able to:
- **See other users**: List active users in town
- **See other users' rigs**: View rig registry with ownership
- **See cross-user activity**: Town-wide agent status, convoy tracking
- **Coordinate work**: Mail between users, shared beads visibility

#### FR-5: User-Scoped Mail Routing

- Mail addresses include user context: `<user>/<rig>/<role>`
- Users can mail each other directly
- Users can mail other users' agents
- Mail permissions: users can read their own mailboxes

#### FR-6: Simplified Mayor Model

- **Single shared Mayor** (not per-user)
- Mayor has town-wide visibility
- Mayor can coordinate across all users' rigs
- Simplifies architecture vs. per-user mayors

#### FR-7: User-Aware Beads

- Beads can be filtered by creator/owner
- Each bead tracks which user created it
- Work delegation can specify target user
- Cross-user bead references allowed

#### FR-8: Session Management

- Tmux sessions include user context in naming
- User can see their own sessions
- Town-wide session view shows all users (for coordination)

### Non-Functional Requirements

#### NFR-1: Simplicity

- Avoid enterprise complexity (LDAP, SSO, RBAC)
- Local authentication sufficient for v1
- Simple user onboarding (detect from git/GitHub)

#### NFR-2: Collaboration

- Users should feel like they're in a shared workspace
- Easy to see what others are working on
- Encourage coordination, not isolation

#### NFR-3: Attribution

- All work clearly attributed to user
- CVs (work history) are user-scoped
- Enables team retrospectives and metrics

#### NFR-4: Performance

- User context propagation should not add significant overhead
- User filtering should be efficient

---

## Proposed Architecture

### Multi-Overseer Hierarchy

```
~/gt/                                    # Shared Gas Town instance
├── .beads/                              # Town-level beads (hq-* prefix)
│   ├── routes.jsonl                     # Beads routing (includes user context)
│   └── mail/                            # Mail storage (per-user mailboxes)
│       ├── alice/                       # Alice's mailbox
│       ├── bob/                         # Bob's mailbox
│       └── shared/                      # Shared announcements
├── mayor/                                # Shared Mayor
│   ├── town.json                        # Town identity
│   ├── users.json                       # User registry (NEW)
│   └── rigs.json                        # Rig registry with ownership
├── settings/                             # Shared settings (accounts, etc.)
│   ├── config.json                      # Global agent config
│   └── accounts.json                    # Shared Claude accounts
├── deacon/                               # Shared Deacon daemon
│   └── dogs/                            # System maintenance dogs
└── <rig-name>/                          # Project rig (owned by one user)
    ├── config.json                      # Rig config WITH owner field
    ├── .beads/                          # Rig beads
    ├── .repo.git/                       # Git repo
    ├── mayor/rig/                       # Mayor's clone
    ├── refinery/rig/                    # Refinery
    ├── witness/                         # Witness
    ├── crew/                            # Crew members
    └── polecats/                        # Polecats
```

### User Registry (`mayor/users.json`)

New file tracking all users in the town:

```json
{
  "version": 1,
  "users": [
    {
      "username": "alice",
      "name": "Alice Smith",
      "email": "alice@example.com",
      "added": "2026-02-06T10:00:00Z",
      "source": "git-config"
    },
    {
      "username": "bob",
      "name": "Bob Jones",
      "email": "bob@example.com",
      "added": "2026-02-06T14:30:00Z",
      "source": "github-cli"
    }
  ]
}
```

### Rig Ownership (`mayor/rigs.json`)

Extended to include owner field:

```json
{
  "version": 1,
  "rigs": [
    {
      "name": "gastown",
      "path": "/home/chris/gt/gastown",
      "owner": "alice",
      "created": "2026-01-15T10:00:00Z"
    },
    {
      "name": "longeye",
      "path": "/home/chris/gt/longeye",
      "owner": "bob",
      "created": "2026-02-01T09:00:00Z"
    }
  ]
}
```

### Agent Identity Format

#### Option A: Prefix with User (Recommended)

```
<user>/<rig>/<role>/<name>

Examples:
- alice/gastown/polecats/Toast
- bob/longeye/crew/joe
- alice/beads/witness
```

**Pros:**
- User is immediately visible
- Sorts by user in listings
- Clear ownership at a glance

**Cons:**
- Changes all existing identity strings
- Longer identities

#### Option B: User as Property (Alternative)

Keep current format but add user context in metadata:

```
Identity: gastown/polecats/Toast
User: alice
```

**Pros:**
- Minimal changes to existing code
- Backwards compatible with identity parsing

**Cons:**
- User not visible in identity string itself
- Requires looking up user separately

**Recommendation:** Option A for clarity and explicitness.

### Mail Routing

#### User-Scoped Mailboxes

Each user has their own mailbox in `.beads/mail/<username>/`:

```
.beads/mail/
├── alice/                       # Alice's mailbox
│   ├── inbox.json
│   └── sent.json
├── bob/                         # Bob's mailbox
│   ├── inbox.json
│   └── sent.json
└── shared/                      # Town announcements
    └── announcements.json
```

#### Mail Address Format

```
<user>/<rig>/<role>          # Specific agent
<user>/<rig>                 # All agents in user's rig
<user>                       # User's personal inbox

Examples:
- alice/gastown/polecats/Toast   # Alice's polecat Toast
- bob/longeye                    # All Bob's longeye agents
- alice                          # Alice's personal mailbox
```

#### Cross-User Mail

```bash
# Alice mails Bob directly
gt mail send bob -s "Question about API" -m "Hey Bob, ..."

# Alice mails Bob's witness
gt mail send bob/longeye/witness -s "Need help" -m "..."

# Mayor broadcasts to all users
gt mail send --broadcast -s "Town update" -m "..."
```

### Mayor Role: Shared vs Per-User

#### Option A: Single Shared Mayor (Recommended)

- One Mayor instance for the entire town
- Mayor coordinates across all users' rigs
- Mayor can see all activity (town-wide visibility)
- Simplifies architecture

**Pros:**
- Simple architecture (one coordinator)
- Natural fit for collaborative model
- Mayor acts as neutral coordinator

**Cons:**
- Mayor sees all users' work (but that's the collaborative model)

#### Option B: Per-User Mayor

- Each user has their own Mayor instance
- User's Mayor only manages their rigs
- Requires inter-Mayor coordination

**Pros:**
- User-scoped coordination
- Could enable more privacy

**Cons:**
- Complex: Mayor-to-Mayor coordination needed
- Doesn't fit collaborative model
- Town-wide operations become complex

**Recommendation:** Option A (shared Mayor) for simplicity and collaboration.

### Beads: User-Scoped vs Shared

#### Proposed Model: Shared with User Context

- **All beads visible to all users** (collaborative model)
- Each bead has `created_by: <user>/<rig>/<role>/<name>`
- Each bead has `owner: <username>` (optional, for filtering)
- Users can query/filter by user: `bd list --user=alice`

#### Why Shared Visibility?

1. **Collaborative model**: Users work together in shared town
2. **Cross-user dependencies**: Alice's work may depend on Bob's
3. **Town-wide coordination**: Mayor and users need full visibility
4. **Simplicity**: No complex permission model needed

#### User-Scoped Operations

While visibility is shared, some operations are user-scoped:

```bash
# See all beads (town-wide)
bd list

# See Alice's beads only
bd list --user=alice

# See beads assigned to Alice's agents
bd list --assignee=alice/*

# Create bead (auto-attributes to current user)
bd create "Fix bug in auth"
# → created_by: alice/gastown/crew/joe
# → owner: alice
```

### Session Naming Convention

Include user context in tmux session names:

#### Option A: User Prefix

```
gt-<user>-<rig>-<agent>

Examples:
- gt-alice-gastown-Toast
- gt-bob-longeye-joe
- gt-mayor (shared)
- gt-deacon (shared)
```

**Pros:**
- Clear user ownership
- Sorts by user in tmux
- Easy to filter user's sessions

**Cons:**
- Longer session names
- Changes existing naming convention

#### Option B: User Suffix

```
gt-<rig>-<agent>-<user>

Examples:
- gt-gastown-Toast-alice
- gt-longeye-joe-bob
- gt-mayor (shared)
- gt-deacon (shared)
```

**Pros:**
- Rig/agent name still prominent
- User context available

**Cons:**
- User not immediately visible
- Sorting less useful

**Recommendation:** Option A (user prefix) for clarity.

### User Authentication and Onboarding

#### Initial User Detection

When a new user first uses Gas Town:

```bash
# User runs: gt install ~/gt
Detected user from git config: Alice Smith <alice@example.com>

You are joining Gas Town at: ~/gt/
This town has 2 existing users: alice, bob

Register as new user? [y/N]: y
Username [alice]: alice
Display name [Alice Smith]:
Email [alice@example.com]:

✓ Added user 'alice' to town
✓ You are now authenticated as: alice
```

#### User Context Propagation

User identity flows through:

1. **Environment variable**: `GT_USER=alice`
2. **Session metadata**: tmux sets `GT_USER` per session
3. **Command context**: All `gt` commands use `GT_USER`
4. **Agent spawning**: Agents inherit user from spawn context

#### User Switching

```bash
# See current user
gt user whoami
Current user: alice

# Switch user (interactive auth)
gt user switch bob
Switching to user: bob
Confirm you are bob [y/N]: y
✓ Switched to user: bob

# List users in town
gt user list
Users in ~/gt/:
  * bob (you)
  alice
```

### Current User Detection

Priority order for determining current user:

1. `GT_USER` environment variable (explicit override)
2. User in current tmux session metadata
3. Most recent user from `~/.gt-current-user`
4. Prompt user to select

---

## Implementation Phases

### Phase 1: User Registry and Identity (Week 1-2)

**Goal:** Establish user identity foundation

**Tasks:**

1. Create user registry system
   - `internal/user/registry.go` - User CRUD operations
   - `mayor/users.json` - Persistent user list
   - User struct: `{username, name, email, added, source}`

2. Implement user detection and onboarding
   - Detect from git config (reuse existing overseer logic)
   - Detect from GitHub CLI
   - Interactive registration for new users
   - Update `gt install` to support multi-user

3. Add user context to rig registry
   - Extend `Rig` struct with `Owner string` field
   - Update `mayor/rigs.json` schema
   - Migration: assign existing rigs to detected user

4. User context propagation
   - `GT_USER` environment variable
   - Tmux session metadata for user context
   - `internal/user/context.go` - Get current user

5. CLI commands for user management
   - `gt user list` - Show all users
   - `gt user whoami` - Show current user
   - `gt user switch <user>` - Switch users
   - `gt user add <user>` - Add new user (manual)

**Deliverables:**
- User registry implementation
- User detection and onboarding flow
- Rig ownership model
- User context propagation
- Basic user CLI commands

**Testing:**
- Unit tests for user registry
- User detection from various sources
- Rig ownership assignment
- User context flow through commands

---

### Phase 2: Agent Identity Update (Week 3-4)

**Goal:** Update agent identity format to include user context

**Tasks:**

1. Update agent identity format
   - Change from `<rig>/<role>/<name>` to `<user>/<rig>/<role>/<name>`
   - Update `internal/agent/identity.go`
   - Update identity parsing/validation

2. Update agent spawning
   - `internal/polecat/manager.go` - Include user in polecat identity
   - `internal/crew/manager.go` - Include user in crew identity
   - Tmux session naming: `gt-<user>-<rig>-<agent>`

3. Update git attribution
   - Git commits: `Author: <user>/<rig>/<role>/<name> <email>`
   - Update commit author formatting
   - Preserve user context in git history

4. Update beads attribution
   - `created_by` field: include user in agent ID
   - `owner` field: extract user from agent ID or explicit
   - Assignee format: `<user>/<rig>/<role>/<name>`

5. Update event attribution
   - Event `actor` field: full user-prefixed identity
   - Town log entries include user context
   - Activity feed shows user attribution

**Deliverables:**
- Updated agent identity format
- User-aware agent spawning
- Updated git and beads attribution
- User context in events and logs

**Testing:**
- Identity parsing with user prefix
- Polecat/crew spawning with user context
- Git commit attribution
- Beads query by user
- Event filtering by user

---

### Phase 3: Mail Routing (Week 5-6)

**Goal:** Enable user-to-user mail and user-scoped mailboxes

**Tasks:**

1. User-scoped mailbox structure
   - Create `.beads/mail/<user>/` directories
   - Separate inbox/sent per user
   - Shared mailbox for announcements

2. Update mail address format
   - Support `<user>/<rig>/<role>/<name>` addresses
   - Support `<user>/<rig>` wildcard (all agents in rig)
   - Support `<user>` (user's personal inbox)
   - Backwards compatibility: parse old format

3. Update mail routing logic
   - `internal/mail/router.go` - User-aware routing
   - User mailbox resolution
   - Permission checks (users can only read their own mail)

4. Update mail CLI commands
   - `gt mail send <user>/<addr>` - Send to user's agent
   - `gt mail send <user>` - Send to user's inbox
   - `gt mail inbox` - Current user's inbox (scoped)
   - `gt mail send --broadcast` - All users

5. Mail notifications
   - Notify users of new mail in their mailbox
   - Dashboard shows per-user unread counts
   - CLI indicator for unread mail

**Deliverables:**
- User-scoped mailbox structure
- Updated mail routing with user context
- User-to-user mail sending
- User mailbox access controls

**Testing:**
- Mail delivery to user-scoped addresses
- User mailbox isolation
- Broadcast mail to all users
- Mail routing with wildcards

---

### Phase 4: UI/UX and Visibility (Week 7-8)

**Goal:** Surface user context in UI and enable cross-user visibility

**Tasks:**

1. Update CLI prompts
   - Show current user: `[alice] gt> `
   - User context in status commands
   - `gt status` shows current user's activity

2. Update agent listings
   - `gt agents` - Show all agents with user attribution
   - `gt agents --user=alice` - Filter by user
   - Rig listings show ownership

3. Update dashboard
   - Per-user activity sections
   - Cross-user activity feed (collaborative view)
   - Rig ownership indicators
   - User presence indicators (who's active)

4. Update beads CLI
   - `bd list --user=alice` - Filter by user
   - `bd stats --group-by=user` - User statistics
   - User-scoped CV: `bd cv alice`

5. Update convoy tracking
   - Convoy owner (user who created it)
   - Cross-user convoy participation
   - Attribution in convoy reports

**Deliverables:**
- User context in CLI prompts and status
- User-filtered agent and bead listings
- Updated dashboard with user views
- Cross-user visibility for coordination

**Testing:**
- CLI shows correct user context
- User filtering works correctly
- Dashboard updates with user activity
- Cross-user visibility without leakage

---

### Phase 5: Documentation and Migration (Week 9-10)

**Goal:** Document multi-overseer model and provide migration path

**Tasks:**

1. Update core documentation
   - `docs/overview.md` - Add multi-overseer concepts
   - `docs/glossary.md` - Define user/overseer terminology
   - New: `docs/multi-overseer-guide.md` - User guide

2. Create migration guide
   - Detect single-overseer installations
   - Assign existing rigs to detected user
   - Update existing agent identities (optional: backfill)
   - `gt migrate multi-overseer` command

3. Update getting started guide
   - Multi-user installation
   - Adding users to existing town
   - User onboarding flow

4. Create best practices guide
   - When to add users vs create rigs
   - Coordinating work between users
   - Mail etiquette and conventions
   - Cross-user dependencies

**Deliverables:**
- Updated documentation
- Migration tool for existing installations
- User guide for multi-overseer model
- Best practices documentation

**Testing:**
- Migration from single-overseer installations
- Documentation accuracy
- User onboarding flow

---

## Technical Design Details

### User Registry Schema

```go
// internal/user/types.go
type User struct {
    Username  string            `json:"username"`  // Unique identifier
    Name      string            `json:"name"`      // Display name
    Email     string            `json:"email,omitempty"`
    Added     time.Time         `json:"added"`
    Source    string            `json:"source"`    // git-config, github-cli, manual
    Metadata  map[string]string `json:"metadata,omitempty"`
}

type UserRegistry struct {
    Version int    `json:"version"`
    Users   []User `json:"users"`
}
```

### Rig Ownership Schema

```go
// internal/rig/types.go
type Rig struct {
    Name      string    `json:"name"`
    Path      string    `json:"path"`
    GitURL    string    `json:"git_url"`
    Owner     string    `json:"owner"`      // NEW: username
    Created   time.Time `json:"created"`
    LocalRepo string    `json:"local_repo,omitempty"`
    Config    *config.BeadsConfig `json:"config,omitempty"`
}
```

### Agent Identity Format

```go
// internal/agent/identity.go
type Identity struct {
    User  string  // username
    Rig   string  // rig name
    Role  string  // polecat, crew, witness, etc.
    Name  string  // agent name (e.g., Toast, joe)
}

func (id *Identity) String() string {
    return fmt.Sprintf("%s/%s/%s/%s", id.User, id.Rig, id.Role, id.Name)
}

// Parse from string: alice/gastown/polecats/Toast
func ParseIdentity(s string) (*Identity, error) {
    parts := strings.Split(s, "/")
    if len(parts) != 4 {
        return nil, fmt.Errorf("invalid identity format: %s", s)
    }
    return &Identity{
        User: parts[0],
        Rig:  parts[1],
        Role: parts[2],
        Name: parts[3],
    }, nil
}
```

### User Context Propagation

```go
// internal/user/context.go
func GetCurrentUser() (string, error) {
    // Priority 1: GT_USER environment variable
    if user := os.Getenv("GT_USER"); user != "" {
        return user, nil
    }

    // Priority 2: Tmux session metadata
    if tmux.InTmux() {
        if user, err := tmux.GetSessionMetadata("GT_USER"); err == nil {
            return user, nil
        }
    }

    // Priority 3: ~/.gt-current-user
    if user, err := loadCurrentUserFile(); err == nil {
        return user, nil
    }

    // Priority 4: Prompt user
    return promptForUser()
}

func SetCurrentUser(user string) error {
    // Set in environment (for current process)
    os.Setenv("GT_USER", user)

    // Set in tmux (if in tmux session)
    if tmux.InTmux() {
        _ = tmux.SetSessionMetadata("GT_USER", user)
    }

    // Save to ~/.gt-current-user (persistent)
    return saveCurrentUserFile(user)
}
```

### Mail Address Resolution

```go
// internal/mail/address.go
type Address struct {
    User  string  // username (required)
    Rig   string  // rig name (optional)
    Role  string  // role (optional)
    Name  string  // agent name (optional)
}

func ParseAddress(s string) (*Address, error) {
    parts := strings.Split(s, "/")
    addr := &Address{User: parts[0]}

    if len(parts) >= 2 {
        addr.Rig = parts[1]
    }
    if len(parts) >= 3 {
        addr.Role = parts[2]
    }
    if len(parts) >= 4 {
        addr.Name = parts[3]
    }

    return addr, nil
}

// Resolve address to mailbox path
func (a *Address) MailboxPath(townRoot string) string {
    if a.Rig == "" {
        // User inbox: alice
        return filepath.Join(townRoot, ".beads", "mail", a.User, "inbox.json")
    } else if a.Role == "" {
        // Rig-wide: alice/gastown (all agents in rig)
        // This is a multi-recipient address, needs expansion
        return ""
    } else {
        // Specific agent: alice/gastown/polecats/Toast
        return filepath.Join(townRoot, ".beads", "mail", a.User, a.Rig, a.Role, a.Name, "inbox.json")
    }
}
```

---

## Migration Strategy

### Automatic Detection & Migration

When a user with an existing single-overseer installation runs `gt` after upgrade:

```bash
$ gt agents

Detected single-overseer Gas Town installation.
Upgrading to multi-overseer support...

Detected overseer: Alice Smith <alice@example.com>
Creating user account for 'alice'...

Assigning existing rigs to 'alice':
  - gastown
  - beads
  - pixelforge

✓ Migration complete! You are now user 'alice'.

To add more users: gt user add <username>
```

### Manual Migration Process

For explicit control:

```bash
# Check current state
gt user status
Current installation: single-overseer (legacy)
Overseer: Alice Smith <alice@example.com>

# Migrate to multi-overseer
gt migrate multi-overseer
Creating user registry...
Detected user: Alice Smith (alice@example.com)

Assign existing rigs to user 'alice'? [y/N]: y
✓ Assigned 3 rigs to user 'alice'

✓ Migration complete

# Add another user
gt user add bob
Display name: Bob Jones
Email: bob@example.com
✓ Added user 'bob'

# Bob creates his first rig
gt rig add longeye git@github.com:example/longeye.git
✓ Created rig 'longeye' (owned by bob)
```

---

## Security Considerations

### User Isolation

**What IS isolated:**
- Mailbox access (users can only read their own mail)
- Rig ownership (only owner can modify rig config)
- Agent spawning (users spawn agents in their own rigs)
- Settings (personal settings in `~/.gt-user-<username>/`)

**What is NOT isolated (intentionally collaborative):**
- Beads visibility (all users see all beads)
- Agent listings (all users see all agents)
- Git repositories (shared access within rig)
- Settings/accounts (shared Claude accounts)

### Authentication Model

v1 authentication is **local and simple**:
- Users identified by username (unique in town)
- No passwords or tokens required
- Trust model: filesystem access = authorized user
- User switching requires confirmation (prevent accidents)

**Future enhancements (out of scope for v1):**
- Password/token authentication
- Session timeouts
- Audit logging for user actions
- External auth (GitHub OAuth, etc.)

### Access Control

**Rig-level permissions:**
- Owner can: modify config, assign work, shutdown agents
- Non-owners can: view status, read beads, coordinate work

**Mail permissions:**
- Users can read their own mailboxes
- Users can send to any user/agent
- No mail interception between users

**Beads permissions:**
- All users can create/read/update any bead (collaborative)
- Attribution tracked via `created_by` field
- Future: owner-only editing for user-scoped beads

---

## Comparison: Multi-Overseer vs Multi-Tenant

| Aspect | Multi-Overseer (This Plan) | Multi-Tenant (Separate Plan) |
|--------|---------------------------|------------------------------|
| **Model** | Collaborative workspace | Isolated environments |
| **Users** | Multiple in one instance | One per tenant instance |
| **Town instances** | Single shared `~/gt/` | Multiple `~/gt/tenants/<name>/` |
| **Rig ownership** | Per-user within town | Per-tenant (isolated) |
| **Visibility** | Users see each other's work | Complete isolation |
| **Mayor** | Shared across users | Per-tenant |
| **Beads** | Shared with attribution | Isolated per tenant |
| **Use case** | Team collaboration | Project isolation, hosting |
| **Complexity** | Simple | Higher (tenant switching, quotas) |

**Both models can coexist:**
- Multi-tenant at the instance level (separate towns)
- Multi-overseer within each tenant instance (collaborative teams)

---

## Open Questions & Decisions

### Q1: Should users be able to delegate rig ownership?

**Options:**
- **A:** Rigs permanently owned by creator (v1)
- **B:** Owner can transfer rig to another user
- **C:** Shared rig ownership (multiple owners)

**Recommendation:** Option A for v1 (simplest). Add transfer in v2 if needed.

**Rationale:**
- Ownership transfer adds complexity (git history, attribution)
- Shared ownership requires permission model
- Can work around with cross-rig worktrees for collaboration

---

### Q2: How granular should mail access control be?

**Options:**
- **A:** Users can only read their own mailbox (strict)
- **B:** Users can read mail sent to their agents
- **C:** Users can read all mail in the town (full collaboration)

**Recommendation:** Option A for v1, Option B as enhancement.

**Rationale:**
- Mailbox privacy prevents accidental info leakage
- Agent-to-agent mail should be visible to owner
- Full visibility not needed for collaboration (use shared beads instead)

---

### Q3: Should there be a "town admin" role?

**Options:**
- **A:** All users have equal privileges (flat model)
- **B:** First user is admin, can manage other users
- **C:** Explicit admin role with elevated privileges

**Recommendation:** Option B (first user is admin) for v1.

**Rationale:**
- Someone needs to add/remove users
- First user (town creator) natural choice
- Can add explicit role system later if needed

**Admin privileges:**
- Add/remove users
- View all mailboxes (for debugging)
- Shutdown town-level agents
- Modify town-wide settings

---

### Q4: How to handle cross-user dependencies in beads?

**Options:**
- **A:** Any user can assign work to any user's agents
- **B:** Users can only assign work to their own agents
- **C:** Users can request work from others (approval needed)

**Recommendation:** Option A for v1 (collaborative model).

**Rationale:**
- Collaborative model encourages coordination
- Cross-user dependencies are natural in teams
- Can add approval workflow later if needed

**Implementation:**
```bash
# Alice creates issue and assigns to Bob's agent
bd create "Fix auth bug" --assignee=bob/longeye/crew/joe

# Bob's agent sees it on their queue
bd ready
# → Shows issues assigned to bob's agents
```

---

## Success Metrics

### Technical Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| User onboarding time | <5 minutes | Time from `gt user add` to first rig |
| User context overhead | <2% | Benchmark agent operations with/without user context |
| Mail delivery latency | <100ms | Time from send to inbox |
| Multi-user session scaling | 10+ users, 50+ agents | Load test with multiple users |

### User Experience Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| User visibility | 100% | Users can see other users' activity |
| Attribution accuracy | 100% | All work correctly attributed to user |
| Onboarding clarity | >4/5 stars | User feedback on onboarding flow |
| Coordination ease | >4/5 stars | User feedback on cross-user workflows |

---

## Risks & Mitigations

### Risk 1: User Confusion (Overseer vs User vs Agent)

**Likelihood:** Medium
**Impact:** Medium

**Mitigation:**
- Clear terminology in docs (overseer = human, agent = AI)
- Consistent naming in UI (always "user", not "overseer" in commands)
- Visual distinction (user names lowercase, agent names capitalized)

---

### Risk 2: Breaking Existing Single-User Installations

**Likelihood:** Low
**Impact:** High

**Mitigation:**
- Automatic migration with single detected user
- Backward compatibility in identity parsing
- Extensive testing on legacy installations
- Rollback capability (keep old format available)

---

### Risk 3: Privacy Concerns with Shared Visibility

**Likelihood:** Low
**Impact:** Medium

**Mitigation:**
- Clear documentation of collaborative model
- User mailbox privacy enforced
- Option to add privacy features later (user-scoped beads, etc.)
- Communicate: "shared workspace" model upfront

---

### Risk 4: User Context Propagation Bugs

**Likelihood:** Medium
**Impact:** Medium

**Mitigation:**
- Comprehensive testing of user context flow
- Explicit `GT_USER` validation at boundaries
- Fallback to prompting if user context lost
- Debug mode: `GT_DEBUG_USER=1` shows context resolution

---

## Future Enhancements

### Short-term (3-6 months)

1. **User preferences**: Per-user settings (theme, notification preferences)
2. **User activity dashboard**: Real-time view of who's working on what
3. **Cross-user notifications**: "@alice can you review this?"
4. **User-scoped beads filters**: `bd list --mine` (my created beads)

### Medium-term (6-12 months)

1. **User authentication**: Optional password/token auth
2. **User presence**: Online/offline indicators
3. **Rig transfer**: Transfer ownership between users
4. **Shared rig ownership**: Multiple owners per rig
5. **User groups**: Group users into teams with shared visibility

### Long-term (12+ months)

1. **Remote collaboration**: Multi-machine Gas Town (users on different hosts)
2. **External auth integration**: GitHub OAuth, LDAP
3. **Enterprise features**: Audit logging, compliance
4. **User quotas**: Limit rigs/agents per user

---

## Appendix A: User Onboarding Flow

### New User Joining Existing Town

```bash
$ gt status
Not authenticated. You are joining Gas Town at: ~/gt/
This town has 2 existing users: alice, bob

Register as new user? [y/N]: y

Username: charlie
Display name [Charlie Brown]:
Email [charlie@example.com]:

✓ Added user 'charlie' to town
✓ You are now authenticated as: charlie

Next steps:
- Create your first rig: gt rig add <name> <git-url>
- See town activity: gt agents
- Send mail to users: gt mail send alice -s "Hello"
```

### Admin Adding User Manually

```bash
$ gt user add bob --name="Bob Jones" --email="bob@example.com"
✓ Added user 'bob' to town

Bob can now authenticate with: gt user switch bob
```

---

## Appendix B: CLI Examples

### User Management

```bash
# Show current user
$ gt user whoami
charlie

# List all users
$ gt user list
Users in ~/gt/:
  * charlie (you)
  alice (admin)
  bob

# Switch users
$ gt user switch alice
Switching to user: alice
✓ Switched to user: alice

# Add new user (admin only)
$ gt user add dave --name="Dave Smith" --email="dave@example.com"
✓ Added user 'dave'
```

### Rig Management with Ownership

```bash
# Create rig (auto-assigned to current user)
$ gt rig add myapp git@github.com:example/myapp.git
✓ Created rig 'myapp' (owned by charlie)

# List rigs with ownership
$ gt rig list
Rigs in ~/gt/:
  * myapp (charlie) - ~/gt/myapp
  gastown (alice) - ~/gt/gastown
  longeye (bob) - ~/gt/longeye
  beads (alice) - ~/gt/beads
```

### Cross-User Mail

```bash
# Mail another user directly
$ gt mail send alice -s "Question" -m "Can you review my PR?"
✓ Sent mail to alice

# Mail another user's agent
$ gt mail send alice/gastown/witness -s "Help" -m "Need guidance"
✓ Sent mail to alice/gastown/witness

# Check my inbox (scoped to current user)
$ gt mail inbox
Inbox: charlie (2 unread)
  1. alice: "Re: Question" - 2 minutes ago
  2. bob: "Meeting notes" - 1 hour ago
```

### User-Filtered Agent Listings

```bash
# See all agents (all users)
$ gt agents
Active agents:
  alice/gastown/polecats/Toast - running (2 hours)
  alice/gastown/witness - running (5 hours)
  bob/longeye/crew/joe - running (30 minutes)
  charlie/myapp/polecats/Worker1 - running (10 minutes)

# See only my agents
$ gt agents --user=charlie
Active agents:
  charlie/myapp/polecats/Worker1 - running (10 minutes)

# See another user's agents
$ gt agents --user=alice
Active agents:
  alice/gastown/polecats/Toast - running (2 hours)
  alice/gastown/witness - running (5 hours)
```

### User-Filtered Beads

```bash
# See all beads
$ bd list
All issues:
  gt-abc (alice) - Fix auth bug
  gt-def (bob) - Add metrics
  gt-xyz (charlie) - Refactor API

# See only my beads
$ bd list --user=charlie
My issues:
  gt-xyz - Refactor API

# See issues assigned to my agents
$ bd list --assignee=charlie/*
Assigned to me:
  gt-abc - Fix auth bug (assigned to charlie/myapp/polecats/Worker1)
```

---

## Appendix C: Code Snippets

### Detecting Current User in Commands

```go
// internal/cmd/root.go
func getCurrentUser(cmd *cobra.Command) (string, error) {
    // Try flag first
    if userFlag, _ := cmd.Flags().GetString("user"); userFlag != "" {
        return userFlag, nil
    }

    // Use context detection
    return user.GetCurrentUser()
}

func requireUser(cmd *cobra.Command) (string, error) {
    u, err := getCurrentUser(cmd)
    if err != nil {
        return "", fmt.Errorf("no user context: %w\nRun 'gt user whoami' or set GT_USER", err)
    }
    return u, nil
}
```

### Creating User-Owned Rig

```go
// internal/rig/manager.go
func (m *Manager) CreateRig(name, gitURL string, owner string) error {
    rig := &Rig{
        Name:    name,
        Path:    filepath.Join(m.townRoot, name),
        GitURL:  gitURL,
        Owner:   owner,
        Created: time.Now(),
    }

    // Create rig directory
    if err := os.MkdirAll(rig.Path, 0755); err != nil {
        return err
    }

    // Add to registry
    return m.registry.Add(rig)
}
```

### Spawning User-Scoped Agent

```go
// internal/polecat/manager.go
func (m *Manager) Spawn(rigName, name string, user string) (*Polecat, error) {
    // Identity includes user
    identity := fmt.Sprintf("%s/%s/polecats/%s", user, rigName, name)

    // Tmux session includes user
    sessionName := fmt.Sprintf("gt-%s-%s-%s", user, rigName, name)

    // Set GT_USER in session environment
    env := map[string]string{
        "GT_USER":   user,
        "BD_ACTOR":  identity,
        "GT_ROLE":   "polecat",
    }

    return m.createSession(sessionName, identity, env)
}
```

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-02-06 | Planning Agent (chrome) | Initial comprehensive plan |

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
2. Create tracking beads for each phase
3. Assign engineering resources
4. Begin Phase 1 implementation

---

*End of Multi-Overseer Architecture Plan*
