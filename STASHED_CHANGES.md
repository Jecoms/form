# Stashed Changes Reference

## Fast Path Implementation (Uncommitted)

**Stash:** `stash@{0}` - "WIP: Fast paths for []string and map[string]string - needs error handling fix"  
**Branch:** `optimize-nested-decode-performance`  
**Status:** ⚠️ Works but breaks `TestDecoder_InvalidSliceIndex`

### To Apply Stashed Changes

```bash
# Switch to the branch
git checkout optimize-nested-decode-performance

# Apply the stash
git stash apply stash@{0}

# Or pop it (removes from stash)
git stash pop stash@{0}
```

### What's in the Stash

**File:** `decoder.go`  
**Changes:** 45 insertions (+45 lines)

#### Change 1: Fast path for `[]string` (Lines ~446-468)

Detects when slice element type is `reflect.String` and bypasses expensive reflection recursion:

```go
// Fast path for []string - avoid recursion
if elemType.Kind() == reflect.String {
    for i := 0; i < len(rd.keys); i++ {
        kv = rd.keys[i]
        if kv.ivalue == -1 {
            continue  // ⚠️ NEEDS FIX: Should call d.setError() here
        }

        // Build the full key and lookup the value directly
        oldLen := len(namespace)
        namespace = append(namespace, kv.searchValue...)
        if arr, ok := d.values[bytesToString(namespace)]; ok && len(arr) > 0 {
            varr.Index(kv.ivalue).SetString(arr[0])
            set = true
        }
        namespace = namespace[:oldLen]
    }

    if set {
        v.Set(varr)
    }
    return
}
```

**Performance Impact:** ~10% improvement (147ms → 132ms for 200 values)  
**Bug:** Missing error reporting for invalid indices

#### Change 2: Fast path for `map[string]string` (Lines ~601-623)

Detects when both map key and value types are strings:

```go
// Fast path for map[string]string - avoid recursion
if elemType.Kind() == reflect.String && keyType.Kind() == reflect.String {
    for i := 0; i < len(rd.keys); i++ {
        kv = rd.keys[i]

        // Build the full key and lookup the value directly
        oldLen := len(namespace)
        namespace = append(namespace, kv.searchValue...)
        if arr, ok := d.values[bytesToString(namespace)]; ok && len(arr) > 0 {
            mp.SetMapIndex(reflect.ValueOf(kv.value), reflect.ValueOf(arr[0]))
            set = true
        }
        namespace = namespace[:oldLen]
    }

    if set && !existing {
        v.Set(mp)
    }
    return
}
```

**Performance Impact:** Included in the ~10% improvement  
**Bug:** May need validation for edge cases (not yet tested)

### Required Fix

The fast path needs to report errors just like the normal path does.

**Current code (buggy):**
```go
if kv.ivalue == -1 {
    continue  // ❌ Silently skips invalid index
}
```

**Fixed code (needed):**
```go
if kv.ivalue == -1 {
    d.setError(namespace, fmt.Errorf("invalid slice index '%s'", kv.value))
    continue
}
```

This matches the behavior of the normal path at lines ~451-453.

### Test Results

**Performance Test (PASSING):**
```bash
$ go test -run TestIssue71NestedPerformance -v
=== RUN   TestIssue71NestedPerformance
    decoder_test.go:2017:     10 decoded values took: 447µs
    decoder_test.go:2017:     50 decoded values took: 8.7ms
    decoder_test.go:2017:    200 decoded values took: 132ms
--- PASS: TestIssue71NestedPerformance (0.14s)
```

**Unit Test (FAILING):**
```bash
$ go test -run TestDecoder_InvalidSliceIndex -v
=== RUN   TestDecoder_InvalidSliceIndex
decoder_test.go:1924 <nil> should not be equal <nil>
--- FAIL: TestDecoder_InvalidSliceIndex (0.00s)
```

### How to Fix and Commit

```bash
# 1. Apply the stash
git stash apply stash@{0}

# 2. Fix the error handling in decoder.go
# Add d.setError() call at line ~451 in the slice fast path
# Verify map fast path has proper validation too

# 3. Test the fix
go test -run TestDecoder_InvalidSliceIndex -v

# 4. Verify all tests pass
go test -v

# 5. Confirm performance maintained
go test -run TestIssue71NestedPerformance -v

# 6. Commit
git add decoder.go
git commit -m "perf: add fast paths for string slices/maps with proper error handling

- Detect []string and map[string]string types
- Bypass expensive reflection recursion with direct Set calls
- Maintain proper error handling for invalid indices
- Performance: ~10% improvement (147ms -> 132ms for 200 values)
- Total improvement: 20.5% from baseline (166ms -> 132ms)"

# 7. Push
git push origin optimize-nested-decode-performance
```

### Performance Analysis

**Before fast paths:** 147ms for 200 values  
**After fast paths:** 132ms for 200 values  
**Improvement:** 10.2% (15ms reduction)

**Why it works:**
- Eliminates recursive `setFieldByType()` calls for common types
- Avoids millions of `reflect.New()` allocations
- Direct `SetString()` / `SetMapIndex()` calls are much faster
- Still uses reflection, but with minimal overhead

**Allocation reduction:**
- Before: ~360K allocations for 100 values
- After: ~340K allocations (5.5% reduction)
- Most allocations still from struct traversal (not yet optimized)

### View Full Diff

```bash
# See what's in the stash
git stash show -p stash@{0}

# Or view specific file
git stash show -p stash@{0} -- decoder.go
```

---

**Created:** October 4, 2025  
**Related:** See `OPTIMIZATION_SUMMARY.md` for full context
