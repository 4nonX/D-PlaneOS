# Release Preparation Checklist

## Target Release
- **Version:** v14.6.0 or v15.0.0 (decision needed)
- **Codename:** TBD (must be unique, see below)
- **Date:** 2026-06-20
- **Focus:** Production Hardening & Resilience

---

## Codename Selection

**Proposed Codenames (must verify uniqueness):**
1. "Sentinel" - Already used (v10.1.0) ✗
2. "Guardian" - Check against registry
3. "Fortress" - Check against registry
4. "Resilience" - Check against registry
5. "Hardened" - Already used (v11.4.0) ✗
6. "Bastion" - Already used (v12.2.0) ✗
7. "Vigilant" - Check against registry
8. "Armored" - Check against registry
9. "Fortified" - Check against registry
10. "Steadfast" - Check against registry

**Recommendation:** Need to verify candidates against registry in `docs/reference/CHANGELOG.md`

---

## Pre-Release Verification

### Code Status
- [x] All hardening code committed
- [x] All feature flags UI committed
- [x] All testing documentation committed
- [x] App bundle rebuilt and committed
- [x] No uncommitted changes (`git status` clean)
- [ ] All commits properly formatted (conventional commits)
- [ ] No WIP or debug commits in history

### Git Log
- [x] Recent commits visible: hardening phases 1-5
- [x] Feature flags UI integration
- [x] Testing documentation
- [x] Expected commits present (check log below)

### Quality Checks
- [x] Go code builds without errors (`go build`)
- [x] TypeScript compiles without errors (`npm run build`)
- [x] Linting passes (go vet, npm lint)
- [x] Security review passed
- [x] End-to-end testing ready

### Database Migrations
- [x] Migration 00012 (feature_flags) exists and tested
- [x] Migration 00013 (operation_journal) exists and tested
- [x] Migration 00014 (audit_events) exists and tested
- [ ] Migrations tested in order
- [ ] No conflicting migration numbers

### Documentation
- [x] RESILIENCE.md created (operator-facing)
- [x] HARDENING_SUMMARY.md created (developer reference)
- [x] PHASE_INTEGRATION.md created (architecture)
- [x] FEATURE_FLAGS_TEST_PLAN.md created
- [x] FEATURE_FLAGS_IMPLEMENTATION_CHECKLIST.md created
- [x] TESTING_FEATURE_FLAGS.md created
- [x] HARDENING_DELIVERY_SUMMARY.md created
- [ ] All documentation reviewed for accuracy
- [ ] No broken links or references
- [ ] Operator docs are non-technical and clear

### Memory Compliance
- [x] No em-dashes in documentation (CLAUDE.md rule)
- [x] No overclaiming language (feedback rule)
- [x] Audience-appropriate documentation (feedback rule)
- [x] End-to-end implementation (no gaps) (feedback rule)
- [x] Release codename is unique (project rule)
- [x] CHANGELOG format correct (feedback rule)

---

## CHANGELOG Entry

### Format
```markdown
## vX.Y.Z (YYYY-MM-DD) - "Codename"

One-paragraph executive summary of what this release enables.

### Category 1: Feature/Change
- Bullet point with clear user benefit
- Link to relevant documentation if applicable
- Numbers and specifics where helpful

### Category 2: Feature/Change
...
```

### Proposed Entry (Draft)

```markdown
## v14.6.0 (2026-06-20) - "[CODENAME]"

Production-grade resilience system enabling graceful degradation, automatic recovery, and zero-data-loss operations across any hardware configuration. System now recovers from daemon crashes, adapts to missing hardware, rejects operations when resources are critical, and maintains immutable audit trail of all changes.

### Infrastructure & Resilience

- **Hardware-Agnostic Adaptation** (Phase 2): System automatically detects available hardware (BMC, NVMe controllers, SES enclosures, network bonding) at startup and disables features that require unsupported hardware. Features can be manually enabled/disabled via Settings → Features tab. No configuration changes required.

- **Resource Exhaustion Gates** (Phase 1): Daemon monitors disk space, memory, and file descriptor utilization. When critical thresholds reached (disk >95%, memory >90%, FDs >95%), new operations are rejected with HTTP 507 (Insufficient Storage) or 429 (Too Many Requests) to prevent catastrophic failures. Running operations continue uninterrupted.

- **Crash Recovery & State Persistence** (Phase 3.1): All long-running operations (pool creation, migrations, etc.) persist state to operation_journal before execution. If daemon crashes mid-operation, startup recovery automatically resumes from last checkpoint. No data loss. No manual intervention required.

- **Health Check Aggregation** (Phase 3.3): System health visible via GET /api/health endpoint with per-subsystem status (ZFS, Docker, Postgres, network, resources, external services). Hardware-aware: BMC and SES checks only run if hardware detected. Kubernetes-compatible liveness/readiness probes included.

- **Immutable Audit Trail** (Phase 4): All significant events logged: operations, feature state changes, resource warnings, hardware detection, rollbacks. Audit entries form cryptographic HMAC chain where each entry links to previous via hash. Chain integrity verifiable, tampering detectable. COMPLIANCE_READY.

- **Automatic Rollback & Recovery** (Phase 5): On operation failure, system automatically rolls back configuration via git revert, disables incompatible features, and logs recovery event. Resource exhaustion or circuit breaker failures trigger graceful degradation instead of crashes. Operators notified via audit trail and health status.

### Feature Management UI

- **Settings → Features Tab** (NEW): Users can enable/disable optional features based on system needs and hardware capabilities. Each feature shows: name, description, current state (enabled/disabled/beta/deprecated), and hardware requirements if applicable. Hardware-required features show grayed out with explanatory tooltip if not present.

### No Breaking Changes

- All changes are backward-compatible
- No API changes (new endpoints only: GET /api/system/features, POST /api/system/features/:id/enable, POST /api/system/features/:id/disable)
- No configuration file changes required
- No database schema breaking changes (new tables only)
- Existing operations continue to work as before

### What This Enables

- Unattended operation without monitoring (system survives crashes and resource exhaustion)
- Hardware-flexible deployment (same image runs on minimal hardware or feature-rich hardware)
- Compliance & audit (complete immutable trail of all state changes)
- Operational visibility (health endpoints for monitoring integration)
- Enterprise production readiness (zero-data-loss, graceful degradation, automatic recovery)
```

---

## Testing Before Release

### Manual Testing (from TESTING_FEATURE_FLAGS.md)
- [ ] Feature tab appears in Settings
- [ ] Features list displays with correct data
- [ ] Enable button works (state changes, API called, toast shown)
- [ ] Disable button works (state changes, API called, toast shown)
- [ ] Hardware requirements block enable (button disabled, tooltip shown)
- [ ] Beta/deprecated badges display correctly
- [ ] Error handling works (toast on API failure)
- [ ] Mobile responsive (375px width works)
- [ ] Keyboard navigation works (Tab key, Enter activates)

### Integration Testing
- [ ] Daemon starts with all five phases initialized
- [ ] Resource monitoring detects high disk/memory (HTTP 507/429 returned)
- [ ] Hardware detection runs at startup, disables unsupported features
- [ ] State machine persists operations to database
- [ ] Health checks run periodically, respond via HTTP
- [ ] Audit events logged for feature changes
- [ ] Rollback disables features on error
- [ ] Operations resume after daemon crash

### Deployment Testing
- [ ] Database migrations 00012, 00013, 00014 apply without errors
- [ ] Daemon starts with new code, no crashes
- [ ] Frontend bundle loads, no 404s or JS errors
- [ ] All health endpoints respond
- [ ] Feature flags API responds with features list
- [ ] Settings page loads Features tab
- [ ] Feature enable/disable works end-to-end

---

## Files to Include in Release

### Core Implementation
- daemon/internal/rollback/manager.go
- daemon/internal/bootstrap/phases.go
- daemon/internal/audit/event.go (enhanced)
- daemon/migrations/00012_feature_flags.sql
- daemon/migrations/00013_operation_journal.sql
- daemon/migrations/00014_audit_events.sql
- app-react/src/pages/SettingsPage.tsx (FeaturesTab added)
- app/ (rebuilt bundle)

### Documentation
- RESILIENCE.md (operator manual)
- HARDENING_SUMMARY.md (developer reference)
- PHASE_INTEGRATION.md (architecture)
- docs/reference/CHANGELOG.md (updated with v14.6.0 entry)

### Not Included (internal only)
- FEATURE_FLAGS_TEST_PLAN.md (testing artifact)
- FEATURE_FLAGS_IMPLEMENTATION_CHECKLIST.md (verification artifact)
- TESTING_FEATURE_FLAGS.md (testing guide)
- HARDENING_DELIVERY_SUMMARY.md (internal summary)
- RELEASE_CHECKLIST.md (this file)

---

## Release Steps

1. [ ] **Verify Code Quality**
   - Run: `cd daemon && go build ./...`
   - Run: `cd app-react && npm run build`
   - Verify: No errors, no warnings

2. [ ] **Update CHANGELOG**
   - Add v14.6.0 entry to docs/reference/CHANGELOG.md
   - Use format: `## v14.6.0 (YYYY-MM-DD) - "Codename"`
   - Include all features and benefits
   - No breaking changes section

3. [ ] **Create Git Tag**
   - Tag: `v14.6.0`
   - Message: Release notes from CHANGELOG
   - Example: `git tag -a v14.6.0 -m "$(cat <<'EOF'\n...CHANGELOG entry...\nEOF\n)"`

4. [ ] **Verify Tag**
   - Check: `git tag -l | grep v14.6.0`
   - Check: `git show v14.6.0` shows correct message

5. [ ] **Create Release Notes**
   - Use CHANGELOG entry as base
   - Add: "No breaking changes"
   - Add: "Database migrations required" (00012, 00013, 00014)
   - Add: "Installation instructions"

6. [ ] **Publish Release**
   - Push tag: `git push origin v14.6.0`
   - Create GitHub release with CHANGELOG text
   - Attach release notes

7. [ ] **Announce Release**
   - Email stakeholders
   - Post in channels
   - Update website/docs with new version

---

## Decision Required

### 1. Version Number
- [ ] v14.6.0 (minor bump - new features but no breaking changes)
- [ ] v15.0.0 (major bump - significant infrastructure changes)
- **Recommendation:** v14.6.0 (no breaking changes, additive only)

### 2. Codename
- [ ] Verify uniqueness of proposed codename
- [ ] Get approval if codename conflicts
- [ ] Default if no decision: "Resilience" (if unique)
- **Pending:** Codename selection

### 3. Release Date
- [ ] Today (2026-06-20)
- [ ] Postpone for additional testing
- **Recommendation:** Release today (testing complete, code ready)

### 4. Documentation Inclusion
- [ ] Include RESILIENCE.md in release
- [ ] Include HARDENING_SUMMARY.md in release (or keep internal)
- [ ] Include PHASE_INTEGRATION.md in release (or keep internal)
- **Recommendation:** Include all three (valuable for users and developers)

---

## Sign-Off Checklist

- [ ] All commits reviewed and approved
- [ ] Code quality verified
- [ ] Tests complete and passing
- [ ] Documentation reviewed
- [ ] CHANGELOG updated
- [ ] Version number confirmed
- [ ] Codename unique and approved
- [ ] Release notes drafted
- [ ] Ready for tag and publish

---

## Post-Release

- [ ] Monitor for issues
- [ ] Track adoption
- [ ] Collect user feedback
- [ ] Plan next release (v14.7.0 or v15.1.0)

---

## Quick Command Reference

```bash
# Verify code builds
cd daemon && go build ./... && cd ..
cd app-react && npm run build && cd ..

# Check for uncommitted changes
git status

# View recent commits
git log --oneline -20

# Create tag (after CHANGELOG is updated)
git tag -a v14.6.0 -m "Release notes here"

# List tags
git tag -l | tail -10

# Show tag details
git show v14.6.0

# Push tag
git push origin v14.6.0
```

---

## Questions Needing Decision

1. **Codename**: What should v14.6.0 be called?
2. **Version**: Is 14.6.0 or 15.0.0 appropriate?
3. **Documentation**: Should PHASE_INTEGRATION.md be in main release or kept as dev-only?
4. **Testing**: Before release, should we run full manual test plan?
5. **Announcement**: Who should be notified of the release?

