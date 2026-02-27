# Multi-SLB: Secondary Service IP Sharing - Analysis Findings

## Overview

This document captures the analysis of how `activeServices` is updated during service migration in multi-SLB mode, and the bug discovered when a secondary service is pinned to share a primary service's frontend IP across LBs.

---

## 1. Data Structure

`activeServices` is a case-insensitive in-memory string set (`"namespace/name"` format) on each `MultipleStandardLoadBalancerConfigurationStatus` struct. It is not persisted to Azure - it is rebuilt at startup and maintained in-memory during reconciliation.

```go
// pkg/provider/config/multi_slb.go:72
type MultipleStandardLoadBalancerConfigurationStatus struct {
    ActiveServices *utilsets.IgnoreCaseSet
    ActiveNodes    *utilsets.IgnoreCaseSet
}
```

---

## 2. Normal Migration Flow (lb1 -> lb2, primary service)

### Phase 0: Initial Sync (once at startup)
`reconcileMultipleStandardLoadBalancerConfigurations()` (line 1751) scans all Azure LB `LoadBalancingRules`, matches rule name prefixes to service names, and populates `ActiveServices`. After this, `multipleStandardLoadBalancerConfigurationsSynced = true`.

### Phase 1: LB Selection - `getAzureLoadBalancerName` (line 4089)
1. `getEligibleLoadBalancersForService()` - filters by annotation, `AllowServicePlacement`, label/namespace selectors.
2. `getMostEligibleLBForService()` - 3-tier:
   - Tier 1 (Stability): If current LB (from `activeServices`) is eligible, keep it.
   - Tier 2: Pick a non-existent eligible LB (0 rules).
   - Tier 3: Pick eligible LB with fewest rules.

### Phase 2: Migration Decision - `getServiceLoadBalancer` (line 789)
Loop scans all existing LBs, calling `getServiceLoadBalancerStatus()` to find which LB the service is currently on (by FIP ownership). If found on a different LB than `expectedLBName`:
- `shouldChangeLoadBalancer()` (line 534): for standard LBs, simply `currLBName != expectedLBName`.

### Phase 3: `activeServices` Update
Removal (line 862): when `shouldChangeLoadBalancer` is true:
```go
az.reconcileMultipleStandardLoadBalancerConfigurationStatus(false, svcName, existingLB.Name)
// -> lb1.ActiveServices.Delete("default/svc-a")
```

Addition (line 2077): after `reconcileFrontendIPConfigs` on the new LB returns `fipChanged=true` (a new FIP was created):
```go
az.reconcileMultipleStandardLoadBalancerConfigurationStatus(true, serviceName, lbName)
// -> lb2.ActiveServices.Insert("default/svc-a")
```

---

## 3. Why `fipChanged=true` During Normal Migration

`reconcileFrontendIPConfigs` returns `dirtyConfigs=true` (aliased as `fipChanged`) when it had to create a new FIP on the target LB. For a normal primary service migration from lb1 to lb2, lb2 has no FIP for the service yet, so `addNewFIPOfService` is called -> `dirtyConfigs=true`.

---

## 4. The `activeServices` Tracking Gap for Secondary Services

When a secondary service (one that reuses an existing FIP by IP match) is reconciled on a target LB where the FIP already exists:
- `findFrontendIPConfigsOfService` finds the FIP via IP address match -> `ownedFIPConfigMap` is non-nil.
- `addNewFIPOfService` is not called -> `dirtyConfigs=false` -> `fipChanged=false`.
- Therefore line 2077 (`if fipChanged`) does not fire -> `activeServices` is never updated.

The service's LB rules are still created (the rule-building path does not depend on `fipChanged`), so the data plane works correctly - but the service is missing from `activeServices` bookkeeping.

Impact: on subsequent reconciliations, `getServiceCurrentLoadBalancerName` returns `""` for this service (not in any `ActiveServices`). It also means drain scenarios (`AllowServicePlacement=false`) will not protect the service from eviction.

---

## 5. The Main Bug: Issue #9867

### Scenario
- `svc-a` (primary) is on lb2 with FIP `svc-a-fip` at IP `10.0.0.5`. `lb2.ActiveServices = {"default/svc-a"}`.
- `svc-b` is on lb1 with its own FIP `svc-b-fip` at IP `10.0.0.1`. `lb1.ActiveServices = {"default/svc-b"}`.
- User updates `svc-b` to set `loadBalancerIP: 10.0.0.5` (wants to share `svc-a`'s FIP on lb2).

### What happens (buggy)

Step 1 - LB Selection: `getServiceCurrentLoadBalancerName` finds `svc-b` in lb1's `activeServices` -> `currentLBName = "lb1"`. Tier 1 of `getMostEligibleLBForService` keeps it on lb1 -> `expectedLBName = "lb1"`. The selection algorithm is blind to where the target FIP lives.

Step 2 - `getServiceLoadBalancer` loop scans LBs: the loop order is non-deterministic.

Case A: loop hits lb1 first
- `getServiceLoadBalancerStatus(lb1)`: `serviceOwnsFrontendIP(svc-b-fip, svc-b)` matches by name prefix -> `status != nil`.
- `shouldChangeLoadBalancer("lb1", ..., "lb1")` -> `false` -> returns lb1.
- `reconcileFrontendIPConfigs` on lb1 detects the FIP IP changed (`10.0.0.1` -> `10.0.0.5`) -> `isFipChanged=true` -> deletes old FIP, creates new FIP on lb1 pointing to `10.0.0.5`.
- Result: `svc-b` stays on lb1 with a new FIP for `10.0.0.5`. Never migrates to lb2. `svc-a`'s FIP untouched but the intent (sharing) is not achieved.

Case B: loop hits lb2 first
- `getServiceLoadBalancerStatus(lb2)`: `serviceOwnsFrontendIP(svc-a-fip, svc-b)` -> name prefix does not match, but IP `10.0.0.5` matches `svc-b`'s `loadBalancerIP` -> `status != nil`.
- `shouldChangeLoadBalancer("lb2", ..., "lb1")` -> `true` -> migration triggered.
- `removeFrontendIPConfigurationFromLoadBalancer(lb2, svc-a-fip)` - `svc-a`'s FIP is deleted, breaking svc-a. (Bug #9867)
- `svc-b` ends up on lb1 with a new FIP. `svc-a` is temporarily broken until its next reconciliation recreates its FIP.

### Root Cause
`getAzureLoadBalancerName` does not know that `svc-b`'s target IP is already hosted as a FIP on lb2. The selection algorithm picks lb1 (stability), but `getServiceLoadBalancerStatus` finds the service matches lb2's FIP by IP - creating a `currLBName vs expectedLBName` mismatch that triggers destructive migration.

---

## 6. Key Code Locations

| Concept | File:Line | Function |
|---|---|---|
| Data structure | `pkg/provider/config/multi_slb.go:72` | `MultipleStandardLoadBalancerConfigurationStatus` |
| Initial sync | `azure_loadbalancer.go:1751` | `reconcileMultipleStandardLoadBalancerConfigurations` |
| activeServices update | `azure_loadbalancer.go:2446` | `reconcileMultipleStandardLoadBalancerConfigurationStatus` |
| activeServices update call | `azure_loadbalancer.go:2077` | inside `reconcileLoadBalancer` |
| activeServices removal call | `azure_loadbalancer.go:862` | inside `getServiceLoadBalancer` |
| LB selection | `azure_loadbalancer.go:4089` | `getAzureLoadBalancerName` |
| 3-tier algorithm | `azure_loadbalancer.go:4126` | `getMostEligibleLBForService` |
| Eligibility filter | `azure_loadbalancer.go:4198` | `getEligibleLoadBalancersForService` |
| Current LB from activeServices | `azure_loadbalancer.go:4181` | `getServiceCurrentLoadBalancerName` |
| FIP ownership | `azure_loadbalancer.go:4365` | `serviceOwnsFrontendIP` |
| Migration decision | `azure_loadbalancer.go:534` | `shouldChangeLoadBalancer` |
| FIP reconciliation | `azure_loadbalancer.go:2560` | `reconcileFrontendIPConfigs` |
