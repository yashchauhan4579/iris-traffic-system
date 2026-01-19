
## Task List

### 1. Add API to Register Workers

- [ ] Define an endpoint in the master to accept worker registration (e.g., `/register_worker`).
- [ ] Store and manage worker info (e.g., ID, host, capacity, status) in the master.
- [ ] Return registration success/duplicate/error responses.

### 2. Create a Worker Client That Auto-Registers to the Master

- [ ] Implement a worker client script/service.
- [ ] On startup, the worker should contact the master’s registration API and register itself (with necessary metadata).
- [ ] Handle retry and error cases (with exponential backoff and logging).
- [ ] Worker should periodically renew its registration/heartbeat.

### 3. Add UI to See Workers

- [ ] Create a UI page to list all registered workers.
- [ ] Display details: Worker ID, status, last seen, workload, etc.
- [ ] Support basic filtering/search.

### 4. Add UI to Assign Load to Worker

- [ ] UI to choose either round-robin assignment or to select a specific worker for camera/task assignment.
- [ ] API on the master for assigning load to a specific worker.
- [ ] UI options to trigger assignment and show current assignments.
- [ ] Visual feedback for assignment status and errors.

### Optional Enhancements

- [ ] Handle worker removal/unregistration and error handling in the master and UI.
- [ ] Secure API and worker authentication.

---

## VCC Analytics Features (Implemented)

### 1. Per-Camera Filtering
- **Functionality**: Users can filter VCC statistics by selecting a specific camera from a dropdown or "All Cameras" for aggregated data.
- **Top Devices**: Clicking on a device in the "Top Devices" list automatically filters the dashboard by that camera.

### 2. Advanced Date-Time Filtering
- **Flexible Picker**: Custom date-time range picker with calendar visualization.
- **Minute-Level Granularity**: Support for minute-level data aggregation (1m, 2m, 5m presets) for high-precision validation.
- **Visual Range**: Calendars visually highlight the selected date range across start and end views for better usability.

### 3. Data Visualization
- **Normalized Display**: Dashboard consistently renders stats (counts, classifications, distributions) regardless of whether single-camera or multi-camera view is selected.
- **Real-Time Integration**: Normalized data flow ensures real-time updates respect the selected filters.



