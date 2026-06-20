# Feature Flags UI Test Plan

## Component: FeaturesTab (Settings → Features tab)

### Test Environment
- Dev server: `http://localhost:3001` (or 3000 if available)
- Backend: Must be running with `/api/system/features` endpoint
- Frontend: Built and running via `npm run dev`

---

## Manual Tests

### Test 1: Component Renders
**Steps:**
1. Start dev server: `npm run dev` in app-react/
2. Navigate to http://localhost:3001 (or 3001)
3. Go to Settings page
4. Click "Features" tab

**Expected:**
- Features tab appears in tab list (after License, before Maintenance)
- Tab icon is "extension" (gear icon)
- No console errors
- Component loads without crashing

**Pass/Fail:** ___

---

### Test 2: Enabled Features Section
**Prerequisite:** Backend returns features with state='stable' or state='beta'

**Steps:**
1. Load Settings → Features tab
2. Observe "Enabled Features" section (if any features are enabled)

**Expected:**
- Section header reads "Enabled Features"
- Each enabled feature shows:
  - Feature name (bold)
  - Description text
  - State badge if beta/deprecated (yellow/red warning)
  - "Disable" button

**Pass/Fail:** ___

---

### Test 3: Available Features Section
**Prerequisite:** Backend returns features with state='disabled'

**Steps:**
1. Load Settings → Features tab
2. Observe "Available Features" section (or "Features" if no enabled)

**Expected:**
- Section header reads "Available Features" (if some enabled) or "Features" (if none enabled)
- Each disabled feature shows:
  - Feature name (bold)
  - Description text
  - Hardware requirement line if `requires_hardware` is set (lighter color)
  - "Enable" button

**Pass/Fail:** ___

---

### Test 4: Hardware Requirements Block Enable
**Prerequisite:** Backend returns feature with `requires_hardware` set (e.g., "BMC")

**Steps:**
1. Load Settings → Features tab
2. Find a feature with hardware requirement (e.g., "HA Clustering" requires BMC)
3. Hover over the feature's Enable button

**Expected:**
- Enable button is DISABLED (grayed out, cursor = not-allowed)
- Tooltip shows: "Your hardware doesn't support this feature"
- Feature card has reduced opacity (0.6)
- Hardware requirement visible: "Requires: BMC"

**Pass/Fail:** ___

---

### Test 5: Enable Feature (No Hardware Requirement)
**Prerequisite:** Feature with state='disabled' and NO `requires_hardware`

**Steps:**
1. Load Settings → Features tab
2. Find an available feature without hardware requirement
3. Click "Enable" button

**Expected:**
- Button shows loading state: "Enabling..."
- Button becomes disabled while loading
- After ~1-2 seconds:
  - Button returns to "Enable" state
  - Feature moves to "Enabled Features" section
  - Success toast shows: "Feature enabled"
  - API POST call to `/api/system/features/:id/enable` succeeds

**Pass/Fail:** ___

---

### Test 6: Disable Feature
**Prerequisite:** Feature with state='stable' or 'beta'

**Steps:**
1. Load Settings → Features tab
2. Find an enabled feature
3. Click "Disable" button

**Expected:**
- Button shows loading state: "Disabling..."
- Button becomes disabled while loading
- After ~1-2 seconds:
  - Button returns to "Disable" state
  - Feature moves to "Available Features" section
  - Success toast shows: "Feature disabled"
  - API POST call to `/api/system/features/:id/disable` succeeds

**Pass/Fail:** ___

---

### Test 7: Beta Feature Warning
**Prerequisite:** Backend returns feature with state='beta'

**Steps:**
1. Load Settings → Features tab
2. Find an enabled feature with state='beta'

**Expected:**
- Feature shows yellow warning badge:
  - Text: "⚠ Beta: Not recommended for production"
  - Color: Warning color (yellow/orange)
  - Placement: Below description

**Pass/Fail:** ___

---

### Test 8: Deprecated Feature Warning
**Prerequisite:** Backend returns feature with state='deprecated'

**Steps:**
1. Load Settings → Features tab
2. Find an enabled feature with state='deprecated'

**Expected:**
- Feature shows red warning badge:
  - Text: "⚠ Deprecated: Use a newer feature instead"
  - Color: Error color (red)
  - Placement: Below description

**Pass/Fail:** ___

---

### Test 9: API Error Handling
**Steps:**
1. Load Settings → Features tab
2. Mock backend to return error on next enable/disable attempt
3. Click Enable or Disable button

**Expected:**
- Loading state shows temporarily
- Error toast appears: "Failed to enable: [error message]" or "Failed to disable: [error message]"
- Button returns to normal state
- Feature state does NOT change
- User can retry

**Pass/Fail:** ___

---

### Test 10: Loading State
**Steps:**
1. Load Settings → Features tab while backend is slow
2. Observe initial load

**Expected:**
- Skeleton loading state shows while fetching features
- After features load, skeleton disappears
- Features list renders

**Pass/Fail:** ___

---

### Test 11: Error State
**Steps:**
1. Load Settings → Features tab while backend returns error

**Expected:**
- Error state displays: "Failed to load features"
- No features list shown
- User sees clear error message

**Pass/Fail:** ___

---

### Test 12: Empty State
**Prerequisite:** Backend returns empty features list

**Steps:**
1. Load Settings → Features tab

**Expected:**
- Message displays: "No features found"
- Centered in a card
- Graceful handling (no blank page)

**Pass/Fail:** ___

---

### Test 13: Tab Navigation
**Steps:**
1. Load Settings page
2. Click Features tab
3. Click another tab (e.g., Security)
4. Click Features tab again

**Expected:**
- Features tab switches to correctly
- State is preserved from previous load (or reloaded from API)
- No console errors
- Smooth tab transitions

**Pass/Fail:** ___

---

### Test 14: Responsive Design
**Steps:**
1. Load Settings → Features tab
2. Resize browser window to mobile size (375px width)
3. Observe layout

**Expected:**
- Features cards stack vertically
- Button stays readable and clickable
- Hardware requirement text wraps
- No overflow or layout breaks
- Touch-friendly button size (48px+ height)

**Pass/Fail:** ___

---

### Test 15: Accessibility
**Steps:**
1. Load Settings → Features tab
2. Use Tab key to navigate buttons
3. Use screen reader to read feature names/descriptions

**Expected:**
- All buttons are keyboard accessible
- Tab order is logical: top to bottom, left to right
- Feature names are announced
- Hardware requirements are described
- Error messages are announced
- Loading states are communicated

**Pass/Fail:** ___

---

## Code Review Checklist

- [ ] FeaturesTab component exists in SettingsPage.tsx
- [ ] Type definitions: Feature, FeaturesResponse exist
- [ ] useQuery hook fetches from `/api/system/features`
- [ ] useMutation hooks for enable/disable with correct endpoints
- [ ] Loading state uses Skeleton component
- [ ] Error state uses ErrorState component
- [ ] Toast notifications on success/error
- [ ] Hardware requirement disables button
- [ ] Beta/deprecated badges render correctly
- [ ] No console errors or warnings
- [ ] No TypeScript errors
- [ ] App bundle rebuilt after code changes
- [ ] Changes committed with proper message

---

## Browser Compatibility

Test in:
- [ ] Chrome/Chromium (latest)
- [ ] Firefox (latest)
- [ ] Safari (latest)
- [ ] Edge (latest)

---

## Summary

**Tests Passed:** ___ / 15
**Code Review:** ___ / 13
**Browser Compatibility:** ___ / 4

**Overall Status:** _____ (PASS / FAIL)

**Notes:**
