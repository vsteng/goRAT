# Push Client Updates Feature

## Overview
This feature allows administrators to push client updates to specific platforms from the dashboard.

## UI Changes

### Settings Tab - Push Client Updates Section
Added a new section in the Settings tab (`/web/templates/dashboard-new.html`) with the following components:

**Update Configuration:**
- Platform selector (dropdown with options for each platform)
- Version input (e.g., "2.0.0")
- Force update toggle (mandatory vs optional)
- Push Update and Clear buttons

**Statistics Display:**
- Total matching clients
- Updates sent successfully
- Failed updates

**Update Log:**
- Real-time log of update operations
- Status (success/failed) for each client
- Timestamps and detailed messages

## Frontend Functions

### JavaScript Functions Added:

1. **`clearUpdateForm()`**
   - Clears all form fields
   - Hides statistics and log display

2. **`pushClientsUpdate()`**
   - Validates platform and version inputs
   - Calls `/api/push-update` endpoint
   - Displays results with statistics and logs
   - Shows success/error messages to user

3. **Updated `loadUpdatePaths()`**
   - Now loads from `/api/settings` endpoint
   - Loads saved update paths for each platform

4. **Updated `saveUpdatePaths()`**
   - Now persists to `/api/settings` endpoint
   - Uses proper API format with `update_path_` prefix

## Backend Changes

### New API Endpoints

#### `GET /api/settings`
- Retrieves all server settings
- No authentication required (uses existing session check)
- Returns: `map[string]string` of all settings

#### `POST /api/settings`
- Saves server settings
- Accepts: `map[string]string` of settings to update
- Returns: Success message

#### `POST /api/push-update` (NEW)
- Sends update commands to clients by platform
- Request body:
  ```json
  {
    "platform": "windows-amd64|linux-amd64|darwin-arm64|all",
    "version": "2.0.0",
    "force": true
  }
  ```
- Returns:
  ```json
  {
    "total_matching": 5,
    "updates_sent": 4,
    "updates_failed": 1,
    "log": [
      {
        "timestamp": "2025-12-11T10:30:45Z",
        "status": "success",
        "client_id": "abc123",
        "message": "Update command sent to hostname"
      }
    ]
  }
  ```

### New Server Functions

#### `ginHandlePushUpdate(c *gin.Context)`
- Main handler for push update API
- Filters connected clients by platform
- Sends update messages to matching clients
- Generates detailed update log with statistics

#### `getPlatformKey(os, arch string) string`
- Converts OS and Architecture to platform key
- Examples:
  - `("windows", "amd64")` → `"windows-amd64"`
  - `("linux", "386")` → `"linux-386"`
  - `("darwin", "arm64")` → `"darwin-arm64"`

#### `buildUpdateURL(platform, version string, store *ClientStore) string`
- Constructs the update URL from settings
- Replaces `{version}` placeholder with actual version
- Example: `https://example.com/clients/windows/amd64/{version}` → `https://example.com/clients/windows/amd64/2.0.0`

#### `ginHandleGetSettings(c *gin.Context)`
- Public wrapper for AdminGetSettingsHandler

#### `ginHandleSaveSettings(c *gin.Context)`
- Public wrapper for AdminSaveSettingsHandler

## Supported Platforms

The following platforms can be targeted:
- `windows-amd64` (Windows 64-bit)
- `windows-386` (Windows 32-bit)
- `linux-amd64` (Linux 64-bit)
- `linux-386` (Linux 32-bit)
- `darwin-amd64` (macOS Intel)
- `darwin-arm64` (macOS Apple Silicon)
- `all` (All platforms)

## Database
Uses existing `server_settings` table:
- Key format: `update_path_<platform-key>`
- Example: `update_path_windows-amd64`, `update_path_linux-amd64`

## Configuration

Each platform requires a base update URL configured in settings:
```
update_path_windows-amd64: https://example.com/clients/windows/amd64/
update_path_linux-amd64: https://example.com/clients/linux/amd64/
```

The URLs should contain a `{version}` placeholder that will be replaced with the actual version number.

## Client-Side Implementation

The server sends update commands as messages to connected clients with type `MsgTypeUpdateStatus` containing:
```json
{
  "command": "update",
  "url": "https://example.com/clients/windows/amd64/2.0.0",
  "version": "2.0.0",
  "force": true
}
```

Clients are expected to implement the update logic to:
1. Download the update from the provided URL
2. Verify the download
3. Install and restart if force is true, or prompt user if false

## Files Modified

1. `/Users/tengbozhang/chrom/web/templates/dashboard-new.html`
   - Added update management UI section
   - Added JavaScript functions for update operations

2. `/Users/tengbozhang/chrom/server/admin_models.go`
   - Added imports: `encoding/json`, `strings`, `time`, `common`
   - Added handler functions: `ginHandlePushUpdate`, `ginHandleGetSettings`, `ginHandleSaveSettings`
   - Added helper functions: `getPlatformKey`, `buildUpdateURL`

3. `/Users/tengbozhang/chrom/server/handlers.go`
   - Added route: `router.GET("/api/settings", s.ginHandleGetSettings)`
   - Added route: `router.POST("/api/settings", s.ginHandleSaveSettings)`
   - Added route: `router.POST("/api/push-update", s.ginHandlePushUpdate)`

## Usage Example

1. Configure update paths in Settings tab:
   - Windows AMD64: `https://updates.example.com/clients/windows/amd64/`
   - Linux AMD64: `https://updates.example.com/clients/linux/amd64/`
   - etc.

2. Click "Save Settings"

3. Go to "Push Client Updates" section

4. Select platform (e.g., `linux-amd64`)

5. Enter version (e.g., `2.0.0`)

6. Choose force update (yes/no)

7. Click "Push Update"

8. Monitor the update log for results

## Error Handling

- Missing platform/version: Returns 400 Bad Request
- Failed to create message: Logged in update log
- Failed to send command: Logged with error details
- Unconfigured platform URL: Marked as failed in log

All errors are reported in the update log displayed to the user.

## Security Considerations

- Only admin users with dashboard access can push updates
- Uses existing session authentication
- Updates are only sent to online clients
- Force flag allows mandatory vs optional updates
