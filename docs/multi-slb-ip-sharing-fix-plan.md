# Fix Plan: Multi-SLB Secondary Service IP Sharing (Issue #9867)

## Problem Summary

In multi-SLB mode, when a secondary service is updated to share a primary service's frontend IP (via `loadBalancerIP`/PIP selection), the current LB selection + migration path can:
- Keep the service on the wrong LB and create a duplicate FIP, or
- Delete the primary service's shared FIP on another LB.

See `docs/multi-slb-ip-sharing-findings.md` for full analysis.

## Refined Approach

Apply role-aware behavior:
- Pinned service (pins target via PIP annotation, LB IP annotation, or `spec.loadBalancerIP`): must not also set LB-name selector annotation; return an error.
- Primary service: reject LB migration only when its primary-owned FIP is referenced by other resources (LB rules from other services, outbound rules, NAT rules/pools).
- Secondary service: make LB selection FIP-aware, make migration/removal ownership-safe and deterministic, and fix `activeServices` bookkeeping.

All code changes are in `pkg/provider/azure_loadbalancer.go` with tests in `pkg/provider/azure_loadbalancer_test.go`.

---

## Change 1: Add explicit pinned-frontend detection

Add a helper used by selection/migration logic:

```go
// hasPinnedFrontendIdentity returns true when the service explicitly pins
// frontend identity via loadBalancerIP/PIP annotations or spec.loadBalancerIP.
func hasPinnedFrontendIdentity(service *v1.Service) bool {
    if len(getServiceLoadBalancerIPs(service)) > 0 {
        return true
    }

    for _, pipName := range getServicePIPNames(service) {
        if pipName != "" {
            return true
        }
    }

    return false
}
```

Why: this captures pinning intent only. It does not try to classify primary vs secondary role. Ownership-based logic (`serviceOwnsFrontendIP` + `isPrimary`) remains the source of truth for migration safety.

---

## Change 2: Reject conflicting LB-name selector with pinned IP/PIP

Add a validation in multi-SLB selection path (`getAzureLoadBalancerName`) before choosing eligible LBs:

- If `hasPinnedFrontendIdentity(service) == true` (pinned by:
  - `service.beta.kubernetes.io/azure-pip-name` (dual-stack variants),
  - `service.beta.kubernetes.io/azure-load-balancer-ipv4` / `...-ipv6`, or
  - `service.Spec.LoadBalancerIP`)
- And the service sets `service.beta.kubernetes.io/azure-load-balancer-configurations`
- Then return an error.

Pseudo-shape:

```go
if az.UseMultipleStandardLoadBalancers() {
    if hasPinnedFrontendIdentity(service) &&
        len(consts.GetLoadBalancerConfigurationsNames(service)) > 0 {
        return "", fmt.Errorf(
            "service %q sets %q while also pinning a target IP/PIP; "+
                "remove the load balancer configuration annotation and let "+
                "the controller select the LB that hosts the target frontend IP",
            getServiceName(service),
            consts.ServiceAnnotationLoadBalancerConfigurations,
        )
    }
    // existing eligible LB selection path
}
```

Why: pinned-IP/PIP semantics already identify the target frontend identity; LB-name selector adds conflicting placement intent and should be rejected explicitly. The error message tells the user how to resolve the conflict.

---

## Change 3: Block migration for any service whose primary-owned FIP is referenced

In `getServiceLoadBalancer`, make migration blocking ownership-based (not annotation-role-based):

- If migration is requested (`shouldChangeLoadBalancer(...) == true`), iterate the FIPs returned by `getServiceLoadBalancerStatus` and call `serviceOwnsFrontendIP` on each.
- For every FIP where `isPrimary == true` (second return value; name-prefix ownership), call `isFrontendIPConfigUnsafeToDelete(currentLB, service, fip.ID)`.
  - If any primary-owned FIP is unsafe (referenced by other services' LB rules, outbound rules, inbound NAT rules, or inbound NAT pools), return a migration error and stop.
  - If none are unsafe, do not block by this rule.
- This uses the existing `isFrontendIPConfigUnsafeToDelete` function (lines 1608-1707) which already checks all four reference types.

This intentionally covers pinned primary services too, because it keys off primary ownership (`isPrimary == true`), not annotation-based role classification.

Pseudo-shape:

```go
if wantLb && az.shouldChangeLoadBalancer(...) {
    for _, fip := range fipConfigs {
        _, isPrimary, _ := az.serviceOwnsFrontendIP(ctx, fip, service)
        if !isPrimary {
            continue
        }
        unsafe, err := az.isFrontendIPConfigUnsafeToDelete(existingLB, service, fip.ID)
        if err != nil {
            return ..., err
        }
        if unsafe {
            return ..., fmt.Errorf(
                "service %q cannot migrate from LB %q to %q: "+
                    "its primary-owned frontend IP configuration %q is referenced by other resources "+
                    "(load balancing rules, outbound rules, or NAT rules/pools); "+
                    "to unblock, either remove services sharing this frontend IP, "+
                    "or adjust LB eligibility to include %q",
                getServiceName(service),
                currLBName,
                expectedLBName,
                ptr.Deref(fip.Name, ""),
                currLBName)
        }
    }
    // migration path continues below
}
```

Effect: migration is blocked whenever the service has a primary-owned FIP that is referenced by other resources. Non-shared services pass through freely.

### 3a. Operational Note: Expected "Stuck" Reconcile State

If eligibility rules make `currLBName` ineligible while Change 3 blocks migration (because the primary-owned FIP is shared), reconciliation will repeatedly fail with the same error until operator action is taken.

This is intentional fail-closed behavior to prevent destructive migration of a shared frontend IP. The remediation is:
- Remove/repoint secondary services sharing the frontend IP, or
- Adjust LB eligibility so the current host LB remains eligible.

---

## Change 4: Secondary-service-safe LB selection and migration

### 4a. FIP-host-aware selection (`getAzureLoadBalancerName`)

Add a helper that scans `existingLBs` to find which LB currently hosts the pinned target FIP:

```go
// findFIPHostingLBName returns the name of the LB that hosts a FIP matching
// the service's pinned target IP/PIP. Returns "" if no match is found.
func (az *Cloud) findFIPHostingLBName(
    ctx context.Context,
    service *v1.Service,
    existingLBs []*armnetwork.LoadBalancer,
    isInternal bool,
) string {
    if !hasPinnedFrontendIdentity(service) {
        return ""
    }

    for _, lb := range existingLBs {
        if isInternalLoadBalancer(lb) != isInternal {
            continue
        }
        for _, fip := range lb.Properties.FrontendIPConfigurations {
            owns, isPrimary, _ := az.serviceOwnsFrontendIP(ctx, fip, service)
            if owns && !isPrimary {
                return trimSuffixIgnoreCase(
                    ptr.Deref(lb.Name, ""), consts.InternalLoadBalancerNameSuffix)
            }
        }
    }
    return ""
}
```

In `getAzureLoadBalancerName`, when `fipHostLB` is found, override `currentLBName` so tier-1 stability in `getMostEligibleLBForService` picks it:

```go
if az.UseMultipleStandardLoadBalancers() {
    eligibleLBs, err := az.getEligibleLoadBalancersForService(ctx, service)
    // ...
    currentLBName := az.getServiceCurrentLoadBalancerName(service)

    fipHostLB := az.findFIPHostingLBName(ctx, service, existingLBs, isInternal)
    if fipHostLB != "" {
        if !stringInSliceFold(fipHostLB, eligibleLBs) {
            // Change 4c: hard error when host LB is ineligible
            return "", fmt.Errorf(
                "service %q targets a frontend IP on LB %q "+
                    "which is not in the eligible set %v; adjust the LB "+
                    "eligibility configuration to include it",
                getServiceName(service), fipHostLB, eligibleLBs)
        }
        currentLBName = fipHostLB
    }

    lbNamePrefix = getMostEligibleLBForService(
        currentLBName, eligibleLBs, existingLBs,
        requiresInternalLoadBalancer(service))
}
```

Case-sensitivity requirement: `findFIPHostingLBName` returns a lowercased name (from `trimSuffixIgnoreCase`), but `eligibleLBs` contains original-case config names. `StringInSlice` uses `==` (case-sensitive) and would silently fail on case mismatches. Use a case-insensitive variant:

```go
// stringInSliceFold is like StringInSlice but uses strings.EqualFold.
func stringInSliceFold(s string, list []string) bool {
    for _, item := range list {
        if strings.EqualFold(s, item) {
            return true
        }
    }
    return false
}
```

### 4b. Ownership-safe migration/removal (`getServiceLoadBalancer`)

The core structural fix. The current migration path (lines 845-887) has two problems:
1. It passes all matched FIPs (primary + secondary owned) to `removeFrontendIPConfigurationFromLoadBalancer`.
2. It `break`s after the first migration, never scanning remaining LBs.

Fix both by splitting FIPs by ownership and changing loop control flow with an explicit mode flag.

Loop mode decision:
- Precompute `scanAllLBs` before entering the LB loop.
- `scanAllLBs = true` iff `findFIPHostingLBName(...) != ""` (service has an existing secondary/IP-PIP match on some LB and order-independence is required).
- `scanAllLBs = false` otherwise.
- Do not use "always continue in multi-SLB". That option is intentionally excluded.

Pseudo-shape:

```go
fipHostLB := az.findFIPHostingLBName(ctx, service, existingLBs, isInternal)
scanAllLBs := fipHostLB != ""

for i := len(existingLBs) - 1; i >= 0; i-- {
    // ... status/fip split logic ...
    if migrated {
        if scanAllLBs {
            continue
        }
        break
    }
}
```

Why reverse iteration: with `scanAllLBs == true`, the loop may continue after `removeLBFromList(&existingLBs, deletedLBName)`. Reverse indexing avoids stale-index panics and skip-on-shift behavior when the slice shrinks mid-loop.

`scanAllLBs` is computed once before loop entry and remains invariant for that reconciliation pass.

FIP ownership split: at each LB match, call `serviceOwnsFrontendIP` on every FIP returned by `getServiceLoadBalancerStatus`. The `isPrimary` return value (second return; `true` when FIP name starts with service's base LB name) determines the split:
- `primaryOwnedFIPs`: `isPrimary == true` - name-prefix-owned by this service.
- `secondaryOwnedFIPs`: `isPrimary == false` - IP/PIP-matched shared FIPs owned by another service.

Migration rules:
1. On migration, remove only `primaryOwnedFIPs` from non-default LBs.
2. Never remove a secondary-owned shared FIP.
3. When `scanAllLBs == true`, use `continue` (scan all LBs) to deterministically clean stale primary-owned FIPs.
4. When `scanAllLBs == false`, preserve the existing `break` behavior.

Loop behavior when `scanAllLBs == true`:
- If LB == `defaultLBName`: remember it as the return LB, do not remove anything, `continue`.
- If LB != `defaultLBName` and `primaryOwnedFIPs` exist: call `removeFrontendIPConfigurationFromLoadBalancer` with only `primaryOwnedFIPs`, `continue`.
- If LB != `defaultLBName` and only `secondaryOwnedFIPs` exist: skip removal, `continue`.
- After the loop: return the remembered default LB.

When `scanAllLBs == false`, keep existing single-hit flow (`break`) after migration.

Verification - order independence:

| Scenario | lb2 scanned first | lb1 scanned first |
|---|---|---|
| lb2 hosts svc-a's FIP (secondary match for svc-b) | Remember lb2 as return LB; skip removal (secondary-owned) | Remove svc-b's primary FIP from lb1; continue |
| lb1 hosts svc-b's old FIP (primary match) | Remove svc-b's primary FIP from lb1 | Remember lb2 as return LB; skip removal |
| Final result | Return lb2; svc-a's FIP untouched | Return lb2; svc-a's FIP untouched |

Both orderings produce the same outcome.

### 4c. Host-LB eligibility is mandatory for secondary sharing

Integrated into Change 4a above. If a service's target shared FIP is found on `hostLB` and `hostLB` is not in `eligibleLBs`, return an error.

- Do not fall back to another LB (would create a duplicate FIP and break sharing).
- Error explicitly mentions service name, target host LB, and eligible LB list.
- This enforces policy consistency: user requested to share a specific existing FIP, so placement must allow that LB.

---

## Change 5: Fix `activeServices` tracking gap

Keep the bookkeeping fix in `reconcileLoadBalancer` (line 2077):

```go
if fipChanged || az.UseMultipleStandardLoadBalancers() {
    az.reconcileMultipleStandardLoadBalancerConfigurationStatus(wantLb, serviceName, lbName)
}
```

Why: secondary services that reuse an existing FIP have `fipChanged=false`; we still need `activeServices` updates in multi-SLB mode. `SafeInsert`/`Delete` are idempotent and mutex-protected, so unconditional calls in multi-SLB mode have no adverse effect.

---

## Files to Modify

- `pkg/provider/azure_loadbalancer.go`
- `pkg/provider/azure_loadbalancer_test.go`

---

## Implementation Notes

### Key function contracts referenced by this plan

| Function | Returns | Used by |
|---|---|---|
| `serviceOwnsFrontendIP(ctx, fip, svc)` | `(owns bool, isPrimary bool, ipVersion)` | Changes 3, 4a, 4b - `isPrimary` drives the ownership split |
| `isFrontendIPConfigUnsafeToDelete(lb, svc, fipID)` | `(unsafe bool, err)` | Change 3 - checks LB rules, outbound, NAT rules/pools |
| `getServiceLoadBalancerStatus(ctx, svc, lb)` | `(status, lbIPs, fipConfigs, err)` | Change 4b - provides the flat FIP list to split |
| `hasPinnedFrontendIdentity(svc)` | `bool` | Changes 2, 4a - pinning intent check |

### Case sensitivity

`trimSuffixIgnoreCase` lowercases its output. `eligibleLBs` from `getEligibleLoadBalancersForService` returns config names in original case. All comparisons between these two sources must use `strings.EqualFold`, not `==`. The new `stringInSliceFold` helper enforces this. The existing `StringInSlice` in `getMostEligibleLBForService` works only because both sides originate from `multiSLBConfig.Name` (same case); it should not be used for cross-source comparisons.

---

## Tests to Add

### `TestHasPinnedFrontendIdentity`
| Case | Expected |
|---|---|
| No `loadBalancerIP`, no PIP annotation | `false` |
| `loadBalancerIP` set | `true` |
| PIP name set (one slot non-empty) | `true` |
| Both PIP slots empty | `false` |
| Primary service that also sets `loadBalancerIP` | `true` |

### `TestGetAzureLoadBalancerName` (extend)
| Case | Expected |
|---|---|
| Secondary with IP/PIP + `azure-load-balancer-configurations` | returns error (Change 2) |
| Secondary tracked in lb1 activeServices, target FIP hosted on eligible lb2 | selects `lb2` |
| Secondary target FIP on lb2 but lb2 ineligible | returns error (Change 4c) |
| Secondary, host LB name differs in case from config name | still selects correctly (`stringInSliceFold`) |
| Primary service | unchanged selection behavior (no host-FIP override path) |

### `TestGetServiceLoadBalancerMultiSLB` (extend)
| Case | Expected |
|---|---|
| Secondary sharing IP, LB order `[lb2, lb1]` | never remove lb2 shared FIP; remove lb1 primary-owned FIP; return lb2 |
| Secondary sharing IP, LB order `[lb1, lb2]` | same final result as above (order-independent) |
| Secondary, host LB ineligible | returns error; no removal on any LB |
| Primary with `currLB != expectedLB`, FIP referenced by other services' LB rules | returns migration error; no removal performed |
| Primary with `currLB != expectedLB`, `loadBalancerIP` set, FIP referenced by other services' LB rules | returns migration error; no removal performed |
| Primary with `currLB != expectedLB`, FIP referenced by outbound rule only | returns migration error (outbound rules also trigger unsafe check) |
| Primary with `currLB != expectedLB`, FIP not referenced | migration proceeds normally |
| `scanAllLBs=true` and one LB is deleted during loop | no panic; no skipped LB processing |

### `TestChange3BlockedMigrationErrorMessage`
| Case | Expected |
|---|---|
| Primary migration blocked by shared primary-owned FIP | error includes `currLB`, `expectedLB`, and remediation guidance |

### `TestReconcileLoadBalancerActiveServices` (extend)
| Case | Expected |
|---|---|
| Secondary service, `fipChanged=false`, multi-SLB | service inserted into `activeServices` |
| Secondary deletion, `fipChanged=false`, multi-SLB | service removed from `activeServices` |

### Targeted run

```bash
go test ./pkg/provider/ -run "TestHasPinnedFrontendIdentity|TestGetAzureLoadBalancerName|TestGetServiceLoadBalancerMultiSLB|TestReconcileLoadBalancer|TestServiceOwnsFrontendIP" -v
```
